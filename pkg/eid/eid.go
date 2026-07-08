// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eid нь eID Mongolia (eidmongolia.mn) identity provider-ийн Relying
// Party (RP) client юм. Энэ template нь RP-ийн үүрэг гүйцэтгэнэ: Smart-ID
// нийцтэй v3 API-аар (ACSP_V2) QR/push нэвтрэлтийг эхлүүлж, session-ийг
// long-poll-оор хүлээж, амжилттай (COMPLETE + endResult=OK) болоход IdP-ийн
// баталгаажуулсан иргэний identity-г (person блок) хүлээн авдаг.
//
// Wire protocol (well-known: https://eidmongolia.mn/.well-known/eid):
//
//	POST {base}/authentication/device-link/anonymous            → QR нэвтрэлт
//	POST {base}/authentication/notification/etsi/PNOMN-{civil}  → РД push нэвтрэлт
//	GET  {base}/session/{sessionID}?timeoutMs=25000             → long-poll төлөв
//	Auth header: Authorization: Bearer <rp_sk_...>  +  body-д relyingPartyUUID/Name
//
// COMPLETE хариу нь зөвхөн state=COMPLETE-г буцаадаг; жинхэнэ терминал үр дүн
// (OK / TIMEOUT / USER_REFUSED* / WRONG_VC) нь result.endResult-д байна. Client
// эдгээрийг template-ийн энгийн төлөв рүү (COMPLETE / EXPIRED / REFUSED) буулгана.
//
// IdP нь TLS-ээр хамгаалагдсан, эрх бүхий (authoritative) эх сурвалж бөгөөд RP
// Bearer secret-ээр танигдана. person блок нь иргэний нэр/civil_id-г кирилл+латин
// хэлбэрээр шууд өгдөг тул сертификат задлах шаардлагагүй. Гарын үсгийг (ACSP_V2
// signature) сертификатын эсрэг шалгах нь ирээдүйн сонголттой хатууруулалт —
// одоогоор COMPLETE+OK-д итгэнэ (өмнөх RP contract-ийн зан төлөвтэй ижил).
package eid

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Terminal төлвүүдийн sentinel алдаанууд.
var (
	// ErrSessionExpired нь session хугацаа дууссан (terminal) үед буцна.
	ErrSessionExpired = errors.New("eid: session expired")
	// ErrSessionRefused нь хэрэглэгч нэвтрэлтийг татгалзсан (terminal) үед буцна.
	ErrSessionRefused = errors.New("eid: session refused")
	// ErrInitiateRejected нь IdP initiate-г 4xx-ээр буцаасан (жишээ нь РД
	// олдсонгүй / буруу формат / RP эрх) үед буцна — дуудагч үүнийг цэвэр
	// 4xx хэрэглэгчийн алдаа болгож буулгана (5xx дотоод алдаанаас ялгаатай).
	ErrInitiateRejected = errors.New("eid: initiate rejected")
)

// Template-ийн энгийн session төлвүүд. eID Mongolia v3 нь state-ээр зөвхөн
// RUNNING/COMPLETE буцаадаг тул терминал бүтэлгүйтлийг (endResult) эдгээр рүү
// буулгана — frontend-ийн EidVerify/LoginForm эдгээрийг хүлээдэг.
const (
	StateComplete = "COMPLETE"
	StateExpired  = "EXPIRED"
	StateRefused  = "REFUSED"
	StateRunning  = "RUNNING"
)

const (
	defaultBase   = "https://eidmongolia.mn/v3"
	defaultRPName = "template-web"
	// defaultCertLevel нь нэвтрэлтэд хүсэх гэрчилгээний ДООД түвшин. Smart-ID-д
	// хүссэн түвшин нь минимум тул ADVANCED нь ADVANCED/QUALIFIED/QSCD бүх
	// гэрчилгээг хүлээн авна — нэвтрэлтийн гэрчилгээ ихэвчлэн ADVANCED тул
	// QUALIFIED шаардвал ийм иргэн нэвтэрч чадахгүй (утсан дээр signature
	// гаргаж чадалгүй "серверт холбогдоход алдаа" өгдөг).
	defaultCertLevel = "ADVANCED"
	maxRespBytes     = 256 << 10
)

// Identity нь IdP-ийн баталгаажуулсан иргэний таних мэдээлэл юм.
type Identity struct {
	NationalID  string
	CivilID     string
	GivenName   string
	Surname     string
	GivenNameEn string
	SurnameEn   string
	FullName    string
	KYCLevel    string
	// Certificate нь login COMPLETE-ийн cert.value (DER)-ээс задлагдсан
	// сертификатын дэлгэрэнгүй. Cert байхгүй/задлагдахгүй бол nil (нэвтрэлт
	// зогсохгүй — зөвхөн нэмэлт мэдээлэл).
	DocumentNumber string
	Certificate    *Certificate
}

// Certificate нь иргэний eID сертификатын нээлттэй хэсэг (X.509-аас задалсан).
type Certificate struct {
	Serial    string
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    string // олгогч CA-ийн subject CN
	KeyType   string // жишээ: "ECDSA P-256", "RSA 2048"
}

// StartResult нь initiate хариуны клиентэд харагдах хэсэг.
type StartResult struct {
	SessionID        string
	VerificationCode string
	ExpiresAt        string
	DeviceLinkURL    string
}

// SessionResult нь session poll-ийн үр дүн. Identity нь зөвхөн COMPLETE+OK үед
// дүүрэн байна.
type SessionResult struct {
	State    string
	Identity *Identity
}

// Representation нь иргэний төлөөлж чадах НЭГ байгууллага
// (GET /v3/organization/representations/etsi/{personEtsi}-ийн нэг элемент).
type Representation struct {
	OrgEtsi     string // NTRMN-...
	OrgRegister string // улсын бүртгэлийн дугаар
	OrgName     string // кирилл нэр
	OrgNameEn   string // латин нэр (сонголттой)
	Role        string // ж: Гүйцэтгэх захирал
	RightType   string // ADMIN | MANAGER
	ValidFrom   *time.Time
	ValidTo     *time.Time // nil = хугацаагүй
}

// OrgAffiliate нь байгууллагыг төлөөлж болох эрх бүхий этгээд (улсын бүртгэлээс).
// RegNo нь хувь хүний РД; eidmongolia иргэний РД-г энэ жагсаалттай тааруулж эрхийг
// баталгаажуулна. Kind (CEO|FOUNDER|STAKEHOLDER) нь rightType-г тодорхойлно.
type OrgAffiliate struct {
	RegNo string
	Role  string
	Kind  string
}

// AddRepresentationInput нь AddRepresentation-д дамжуулах, XYP-ээс баталгаажсан
// байгууллагын мэдээлэл + эрх бүхий этгээдийн жагсаалт.
type AddRepresentationInput struct {
	OrgRegister string
	OrgName     string
	OrgNameEn   string
	Affiliates  []OrgAffiliate
}

// Signer нь байгууллагыг төлөөлж / гарын үсэг зурж чадах нэг иргэн.
type Signer struct {
	PersonEtsi string
	RegNo      string
	Name       string
	NameEn     string
	Role       string
	RightType  string // ADMIN | MANAGER
	Status     string // ACTIVE | PENDING (sign-push баталгаажуулалт хүлээж буй)
	Source     string
	Self       bool // нэвтэрсэн хэрэглэгч өөрөө эсэх
}

// OrgConfirmation нь MANAGER нэмэхэд тэр хүн рүү илгээгдсэн eID sign-push
// баталгаажуулалтын session — тэр хүн утсаараа PIN-ээ зурж зөвшөөрөх хүртэл rep
// нь PENDING (хүчингүй) хэвээр.
type OrgConfirmation struct {
	OrgRegister string
	OrgName     string
	SignerEtsi  string
	SignerRegNo string
	SessionID   string
}

// SignersResult нь байгууллагын гарын үсэг зурагчдын жагсаалт + (шинээр нэмэх үед)
// хүлээгдэж буй sign-push баталгаажуулалт.
type SignersResult struct {
	Signers             []Signer
	PendingConfirmation *OrgConfirmation
}

// AddSignerInput нь AddSigner-д дамжуулах шинэ гарын үсэг зурагчийн мэдээлэл.
// Нэмэгдэх гарын үсэг зурагчийн эрх нь ҮРГЭЛЖ MANAGER (eidmongolia талд шийдэгдэнэ)
// тул rightType дамжуулахгүй.
type AddSignerInput struct {
	SignerRegNo string
	Role        string
}

// Client нь eID RP урсгалуудын хийсвэрлэл — тестэд хуурамчаар тавихад хялбар.
type Client interface {
	// QRInitiate нь QR нэвтрэлтийг эхлүүлж session мэдээллийг буцаана. callbackURL
	// нь энэ template-ийн cross-device QR урсгалд ашиглагддаггүй (RP өөрөө poll
	// хийнэ) тул зөвхөн интерфейсийн нийцлийн төлөө үлдээв.
	QRInitiate(ctx context.Context, displayText, callbackURL, nonce string) (*StartResult, error)
	// Initiate нь иргэний РД (civil_id)-аар нэвтрэлтийг эхлүүлнэ — IdP нь тухайн
	// РД-тэй холбоотой бүртгэлтэй төхөөрөмж рүү баталгаажуулах push мэдэгдэл
	// илгээдэг. device_link шаардлагагүй тул хариунд DeviceLinkURL хоосон.
	Initiate(ctx context.Context, nationalID, displayText, callbackURL string) (*StartResult, error)
	// Session нь session-ийн төлвийг long-poll-оор асууна (timeoutMs хүртэл).
	Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error)
	// Representations нь тухайн хүн (personEtsi = PNOMN-<civil_id>)-ий төлөөлж
	// чадах идэвхтэй байгууллагуудыг буцаана. Иргэн байгууллага төлөөлдөггүй
	// бол хоосон slice.
	Representations(ctx context.Context, personEtsi string) ([]Representation, error)
	// AddRepresentation нь улсын бүртгэлээс (XYP) баталгаажуулсан байгууллагыг
	// иргэнд холбоно (ORG_LINK_WRITE эрх шаардана). Иргэний РД нь affiliates
	// (ceo/founders/stakeholders) жагсаалтад байвал л (эрх бүхий) төлөөлөл
	// нэмэгдэнэ — эс бөгөөс ErrNotRepresentative. Иргэний бүх төлөөллийг буцаана.
	AddRepresentation(ctx context.Context, personEtsi string, in AddRepresentationInput) ([]Representation, error)
	// RemoveRepresentation нь иргэн (personEtsi) өөрийн байгууллагын (orgRegister)
	// төлөөллөө цуцлана. Иргэний үлдсэн төлөөллийг буцаана.
	RemoveRepresentation(ctx context.Context, personEtsi, orgRegister string) ([]Representation, error)
	// OrgSigners нь байгууллагын гарын үсэг зурагчдыг буцаана. actingPersonEtsi нь
	// тухайн байгууллагын төлөөлөгч байх ёстой (эс бол ErrNotRepresentative).
	OrgSigners(ctx context.Context, orgRegister, actingPersonEtsi string) ([]Signer, error)
	// AddSigner нь байгууллагад өөр eID иргэнийг (РД) гарын үсэг зурах эрхтэй
	// (MANAGER) төлөөлөгч болгож нэмнэ. Тэр хүн рүү sign-push илгээж, PENDING rep
	// үүсгэнэ. Шинэ жагсаалт + хүлээгдэж буй баталгаажуулалтыг буцаана.
	AddSigner(ctx context.Context, orgRegister, actingPersonEtsi string, in AddSignerInput) (*SignersResult, error)
	// RemoveSigner нь байгууллагаас гарын үсэг зурагчийг (РД) хасна.
	RemoveSigner(ctx context.Context, orgRegister, actingPersonEtsi, signerRegNo string) ([]Signer, error)
	// ResendSigner нь баталгаажаагүй (PENDING) гарын үсэг зурагч руу sign-push-ийг
	// дахин илгээнэ. Шинэ жагсаалт + хүлээгдэж буй баталгаажуулалтыг буцаана.
	ResendSigner(ctx context.Context, orgRegister, actingPersonEtsi, signerRegNo string) (*SignersResult, error)
	// UpdateOrgNameLatin нь байгууллагын латин нэрийг засна (зөвхөн ADMIN). Иргэний
	// шинэ төлөөллийг буцаана.
	UpdateOrgNameLatin(ctx context.Context, orgRegister, actingPersonEtsi, nameLatin string) ([]Representation, error)

	// Person* нь иргэн өөрийн PKI самбарын endpoint-ууд — PKI_READ эрхтэй RP-д
	// л нээгдэнэ (эрхгүй бол ErrPKINotPermitted).
	PersonSummary(ctx context.Context, personEtsi string) (*PersonSummary, error)
	PersonCertificates(ctx context.Context, personEtsi string) (*PersonCertificates, error)
	PersonDevices(ctx context.Context, personEtsi string) (*PersonDevices, error)
	PersonActivity(ctx context.Context, personEtsi string, limit, offset int) (*PersonActivity, error)
}

// client нь eID Mongolia v3 RP API руу залгах HTTP client.
type client struct {
	base      string
	rpUUID    string
	rpName    string
	secret    string
	certLevel string
	http      *http.Client
}

// NewClient нь eID Mongolia RP client үүсгэнэ. base/rpName/certLevel хоосон бол
// өгөгдмөл утга авна (certLevel default = ADVANCED, нэвтрэлтэд хамгийн нийцтэй).
// rpUUID/secret нь оператороос олгогдсон RP таних мэдээлэл — secret нь
// Authorization: Bearer header-т явна, log-д гарахгүй. Poll нь 25с хүртэл
// long-poll хийдэг тул HTTP timeout-ийг 30с болгов.
func NewClient(base, rpUUID, rpName, secret, certLevel string) Client {
	if base == "" {
		base = defaultBase
	}
	if rpName == "" {
		rpName = defaultRPName
	}
	if certLevel == "" {
		certLevel = defaultCertLevel
	}
	return &client{
		base:      strings.TrimRight(base, "/"),
		rpUUID:    rpUUID,
		rpName:    rpName,
		secret:    secret,
		certLevel: certLevel,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// interaction нь eID апп-ийн баталгаажуулах дэлгэцэнд харагдах Smart-ID v3
// interaction (одоогоор displayTextAndPIN). displayText60 нь дээд 60 тэмдэгт.
type interaction struct {
	Type          string `json:"type"`
	DisplayText60 string `json:"displayText60,omitempty"`
}

// authInitiateBody нь auth initiate (device-link + notification)-ийн ACSP_V2
// хүсэлтийн their. АНХААР: auth-д challenge талбар нь `rpChallenge` (base64
// nonce) — sign-ийн `digest`/`hashType`-ээс ялгаатай. Буруу `hash` талбар
// илгээвэл сервер rpChallenge-ийг хоосон гэж үзэж, PIN үед ACSP payload
// эвдэрч "боловсруулах алдаа" өгдөг. interactions нь заавал (апп дэлгэцэнд
// харуулах текст).
type authInitiateBody struct {
	RelyingPartyUUID  string        `json:"relyingPartyUUID"`
	RelyingPartyName  string        `json:"relyingPartyName"`
	CertificateLevel  string        `json:"certificateLevel"`
	SignatureProtocol string        `json:"signatureProtocol"`
	RPChallenge       string        `json:"rpChallenge"`
	Interactions      []interaction `json:"interactions"`
	// InitialCallbackURL — SAME-DEVICE (mobile browser App2App) буцах URL. Хоосон бол CROSS-DEVICE
	// (desktop QR/push): eID backend утас руу callback дамжуулахгүй, browser өөрөө poll хийнэ.
	// eID backend үүнийг өөрийн стандарт зам (/auth/eid/callback) руу force-normalize хийдэг.
	InitialCallbackURL string `json:"initialCallbackUrl,omitempty"`
}

func (c *client) newAuthBody(displayText, callbackURL string) (authInitiateBody, error) {
	challenge, err := randomHashB64()
	if err != nil {
		return authInitiateBody{}, err
	}
	dt := displayText
	if dt == "" {
		dt = c.rpName
	}
	if len(dt) > 60 {
		dt = dt[:60]
	}
	return authInitiateBody{
		RelyingPartyUUID:   c.rpUUID,
		RelyingPartyName:   c.rpName,
		CertificateLevel:   c.certLevel,
		SignatureProtocol:  "ACSP_V2",
		RPChallenge:        challenge,
		Interactions:       []interaction{{Type: "displayTextAndPIN", DisplayText60: dt}},
		InitialCallbackURL: callbackURL,
	}, nil
}

// QRInitiate — device-link auth эхлүүлнэ. callbackURL хоосон бол CROSS-DEVICE (desktop QR); хоосон
// биш бол SAME-DEVICE (mobile browser App2App — утас approve-ийн дараа browser-ийг буцаана).
func (c *client) QRInitiate(ctx context.Context, displayText, callbackURL, _ string) (*StartResult, error) {
	body, err := c.newAuthBody(displayText, callbackURL)
	if err != nil {
		return nil, fmt.Errorf("eid: build challenge: %w", err)
	}
	raw, status, err := c.post(ctx, "/authentication/device-link/anonymous", body)
	if err != nil {
		return nil, err
	}
	if aerr := checkInitiateStatus(raw, status); aerr != nil {
		return nil, aerr
	}
	var out struct {
		SessionID      string          `json:"sessionID"`
		SessionToken   string          `json:"sessionToken"`
		DeviceLinkBase string          `json:"deviceLinkBase"`
		VC             json.RawMessage `json:"vc"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil || out.SessionID == "" {
		return nil, fmt.Errorf("eid initiate: empty/invalid sessionID: %s", snippet(raw))
	}
	// QR-д кодлох агуулга нь ЗҮГЭЭР session UUID — eID Mongolia-гийн ажилладаг
	// demo (eidmongolia.mn/demo) QR-даа sessionID-г шууд тавьдаг (device-link URL
	// БИШ). Апп-ийн QR scanner UUID-г session ID гэж тайлбарлаж, өөрийн серверт
	// (/v3/mobile/session/{id}) резолв хийнэ. `https://…/dl?deviceLinkType=…` URL
	// тавьбал апп задалж чадалгүй унадаг.
	return &StartResult{
		SessionID:        out.SessionID,
		VerificationCode: parseVC(out.VC),
		DeviceLinkURL:    out.SessionID,
	}, nil
}

// Initiate — РД (national ID)-аар push нэвтрэлт эхлүүлнэ. callbackURL хоосон бол CROSS-DEVICE
// (desktop browser + утас руу push — browser өөрөө poll хийнэ); хоосон биш бол SAME-DEVICE
// (утасны browser — push ижил утас руу ирж, approve хийсний дараа eID app browser-ийг callback
// руу буцаана). eID backend callbackURL-ийг стандарт зам (/auth/eid/callback) руу normalize хийнэ.
func (c *client) Initiate(ctx context.Context, nationalID, displayText, callbackURL string) (*StartResult, error) {
	body, err := c.newAuthBody(displayText, callbackURL)
	if err != nil {
		return nil, fmt.Errorf("eid: build challenge: %w", err)
	}
	// РД push: semanticsIdentifier нь ETSI EN 319 412-1 дагуу хувь хүнд
	// PNOMN-<civil_id>. IdP тухайн иргэний бүртгэлтэй төхөөрөмж рүү push хийнэ.
	path := "/authentication/notification/etsi/PNOMN-" + url.PathEscape(strings.TrimSpace(nationalID))
	raw, status, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	if aerr := checkInitiateStatus(raw, status); aerr != nil {
		return nil, aerr
	}
	var out struct {
		SessionID string          `json:"sessionID"`
		VC        json.RawMessage `json:"vc"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil || out.SessionID == "" {
		return nil, fmt.Errorf("eid initiate: empty/invalid sessionID: %s", snippet(raw))
	}
	return &StartResult{
		SessionID:        out.SessionID,
		VerificationCode: parseVC(out.VC),
	}, nil
}

func (c *client) Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error) {
	if sessionID == "" {
		return nil, errors.New("eid: empty session_id")
	}
	path := fmt.Sprintf("/session/%s?timeoutMs=%d", url.PathEscape(sessionID), timeoutMs)
	raw, status, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid session: status %d: %s", status, snippet(raw))
	}
	var out struct {
		State  string `json:"state"`
		Result *struct {
			EndResult      string `json:"endResult"`
			DocumentNumber string `json:"documentNumber"`
		} `json:"result"`
		Cert *struct {
			Value            string `json:"value"` // base64 DER — иргэний сертификат
			CertificateLevel string `json:"certificateLevel"`
		} `json:"cert"`
		Person *struct {
			GivenName   string `json:"givenName"`
			Surname     string `json:"surname"`
			GivenNameEn string `json:"givenNameEn"`
			SurnameEn   string `json:"surnameEn"`
			CivilID     string `json:"civilId"`
			RegNo       string `json:"regNo"`
		} `json:"person"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil || out.State == "" {
		return nil, fmt.Errorf("eid session: invalid response: %s", snippet(raw))
	}

	// state=RUNNING → хараахан дуусаагүй.
	if out.State != "COMPLETE" {
		return &SessionResult{State: StateRunning}, nil
	}

	// COMPLETE: жинхэнэ үр дүн endResult-д. OK биш бол EXPIRED/REFUSED рүү буулгана.
	endResult := ""
	if out.Result != nil {
		endResult = out.Result.EndResult
	}
	if endResult != "OK" {
		if endResult == "TIMEOUT" {
			return &SessionResult{State: StateExpired}, nil
		}
		// USER_REFUSED*, WRONG_VC, DOCUMENT_UNUSABLE, гэх мэт — татгалзсан гэж үзнэ.
		return &SessionResult{State: StateRefused}, nil
	}

	if out.Person == nil {
		return nil, fmt.Errorf("eid session: COMPLETE+OK without person block: %s", snippet(raw))
	}
	id := &Identity{
		CivilID:     out.Person.CivilID,
		NationalID:  out.Person.RegNo,
		GivenName:   out.Person.GivenName,
		Surname:     out.Person.Surname,
		GivenNameEn: out.Person.GivenNameEn,
		SurnameEn:   out.Person.SurnameEn,
	}
	if out.Cert != nil {
		id.KYCLevel = out.Cert.CertificateLevel
		// cert.value байвал X.509-ийг задлан нээлттэй хэсгийг авна. Алдаа гарвал
		// зүгээр алгасна — нэвтрэлт зогсохгүй (cert нь зөвхөн нэмэлт мэдээлэл).
		id.Certificate = parseCertificate(out.Cert.Value)
	}
	if out.Result != nil {
		id.DocumentNumber = out.Result.DocumentNumber
	}
	return &SessionResult{State: StateComplete, Identity: id}, nil
}

func (c *client) Representations(ctx context.Context, personEtsi string) ([]Representation, error) {
	if strings.TrimSpace(personEtsi) == "" {
		return nil, errors.New("eid: empty personEtsi")
	}
	path := "/organization/representations/etsi/" + url.PathEscape(strings.TrimSpace(personEtsi))
	raw, status, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return []Representation{}, nil // хүн олдсонгүй / байгууллага төлөөлдөггүй
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid representations: status %d: %s", status, snippet(raw))
	}
	var out struct {
		Representations []struct {
			OrgEtsi     string     `json:"orgEtsi"`
			OrgRegister string     `json:"orgRegister"`
			OrgName     string     `json:"orgName"`
			OrgNameEn   string     `json:"orgNameEn"`
			Role        string     `json:"role"`
			RightType   string     `json:"rightType"`
			ValidFrom   *time.Time `json:"validFrom"`
			ValidTo     *time.Time `json:"validTo"`
		} `json:"representations"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil {
		return nil, fmt.Errorf("eid representations: invalid response: %s", snippet(raw))
	}
	reps := make([]Representation, 0, len(out.Representations))
	for _, r := range out.Representations {
		reps = append(reps, Representation{
			OrgEtsi: r.OrgEtsi, OrgRegister: r.OrgRegister, OrgName: r.OrgName, OrgNameEn: r.OrgNameEn,
			Role: r.Role, RightType: r.RightType, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo,
		})
	}
	return reps, nil
}

func (c *client) AddRepresentation(ctx context.Context, personEtsi string, in AddRepresentationInput) ([]Representation, error) {
	if strings.TrimSpace(personEtsi) == "" {
		return nil, errors.New("eid: empty personEtsi")
	}
	if strings.TrimSpace(in.OrgRegister) == "" {
		return nil, errors.New("eid: empty orgRegister")
	}
	type affiliate struct {
		RegNo string `json:"regNo"`
		Role  string `json:"role,omitempty"`
		Kind  string `json:"kind,omitempty"`
	}
	body := struct {
		OrgRegister string      `json:"orgRegister"`
		OrgName     string      `json:"orgName"`
		OrgNameEn   string      `json:"orgNameEn,omitempty"`
		Affiliates  []affiliate `json:"affiliates"`
	}{OrgRegister: strings.TrimSpace(in.OrgRegister), OrgName: strings.TrimSpace(in.OrgName), OrgNameEn: strings.TrimSpace(in.OrgNameEn)}
	for _, a := range in.Affiliates {
		body.Affiliates = append(body.Affiliates, affiliate{RegNo: a.RegNo, Role: a.Role, Kind: a.Kind})
	}
	path := "/organization/representations/etsi/" + url.PathEscape(strings.TrimSpace(personEtsi))
	raw, status, err := c.post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid add representation: status %d: %s", status, snippet(raw))
	}
	var out struct {
		Representations []struct {
			OrgEtsi     string     `json:"orgEtsi"`
			OrgRegister string     `json:"orgRegister"`
			OrgName     string     `json:"orgName"`
			OrgNameEn   string     `json:"orgNameEn"`
			Role        string     `json:"role"`
			RightType   string     `json:"rightType"`
			ValidFrom   *time.Time `json:"validFrom"`
			ValidTo     *time.Time `json:"validTo"`
		} `json:"representations"`
	}
	if jErr := json.Unmarshal(raw, &out); jErr != nil {
		return nil, fmt.Errorf("eid add representation: invalid response: %s", snippet(raw))
	}
	reps := make([]Representation, 0, len(out.Representations))
	for _, r := range out.Representations {
		reps = append(reps, Representation{
			OrgEtsi: r.OrgEtsi, OrgRegister: r.OrgRegister, OrgName: r.OrgName, OrgNameEn: r.OrgNameEn,
			Role: r.Role, RightType: r.RightType, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo,
		})
	}
	return reps, nil
}

// parseRepresentations нь representations хариуг []Representation болгож задлана.
func parseRepresentations(raw []byte) ([]Representation, error) {
	var out struct {
		Representations []struct {
			OrgEtsi     string     `json:"orgEtsi"`
			OrgRegister string     `json:"orgRegister"`
			OrgName     string     `json:"orgName"`
			OrgNameEn   string     `json:"orgNameEn"`
			Role        string     `json:"role"`
			RightType   string     `json:"rightType"`
			ValidFrom   *time.Time `json:"validFrom"`
			ValidTo     *time.Time `json:"validTo"`
		} `json:"representations"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("eid representations: invalid response: %s", snippet(raw))
	}
	reps := make([]Representation, 0, len(out.Representations))
	for _, r := range out.Representations {
		reps = append(reps, Representation{
			OrgEtsi: r.OrgEtsi, OrgRegister: r.OrgRegister, OrgName: r.OrgName, OrgNameEn: r.OrgNameEn,
			Role: r.Role, RightType: r.RightType, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo,
		})
	}
	return reps, nil
}

// parseSignersResult нь signers хариуг (жагсаалт + pendingConfirmation) задлана.
func parseSignersResult(raw []byte) (*SignersResult, error) {
	var out struct {
		Signers []struct {
			PersonEtsi string `json:"personEtsi"`
			RegNo      string `json:"regNo"`
			Name       string `json:"name"`
			NameEn     string `json:"nameEn"`
			Role       string `json:"role"`
			RightType  string `json:"rightType"`
			Status     string `json:"status"`
			Source     string `json:"source"`
			Self       bool   `json:"self"`
		} `json:"signers"`
		PendingConfirmation *struct {
			OrgRegister string `json:"orgRegister"`
			OrgName     string `json:"orgName"`
			SignerEtsi  string `json:"signerEtsi"`
			SignerRegNo string `json:"signerRegNo"`
			SessionID   string `json:"sessionId"`
		} `json:"pendingConfirmation"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("eid signers: invalid response: %s", snippet(raw))
	}
	res := &SignersResult{Signers: make([]Signer, 0, len(out.Signers))}
	for _, s := range out.Signers {
		res.Signers = append(res.Signers, Signer{
			PersonEtsi: s.PersonEtsi, RegNo: s.RegNo, Name: s.Name, NameEn: s.NameEn,
			Role: s.Role, RightType: s.RightType, Status: s.Status, Source: s.Source, Self: s.Self,
		})
	}
	if pc := out.PendingConfirmation; pc != nil {
		res.PendingConfirmation = &OrgConfirmation{
			OrgRegister: pc.OrgRegister, OrgName: pc.OrgName, SignerEtsi: pc.SignerEtsi,
			SignerRegNo: pc.SignerRegNo, SessionID: pc.SessionID,
		}
	}
	return res, nil
}

// parseSigners нь signers хариуг []Signer болгож задлана (pending-г үл тооно).
func parseSigners(raw []byte) ([]Signer, error) {
	r, err := parseSignersResult(raw)
	if err != nil {
		return nil, err
	}
	return r.Signers, nil
}

func (c *client) RemoveRepresentation(ctx context.Context, personEtsi, orgRegister string) ([]Representation, error) {
	if strings.TrimSpace(personEtsi) == "" || strings.TrimSpace(orgRegister) == "" {
		return nil, errors.New("eid: empty personEtsi/orgRegister")
	}
	path := "/organization/representations/etsi/" + url.PathEscape(strings.TrimSpace(personEtsi)) + "/" + url.PathEscape(strings.TrimSpace(orgRegister))
	raw, status, err := c.del(ctx, path)
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid unlink: status %d: %s", status, snippet(raw))
	}
	return parseRepresentations(raw)
}

func (c *client) OrgSigners(ctx context.Context, orgRegister, actingPersonEtsi string) ([]Signer, error) {
	raw, status, err := c.signersReq(ctx, http.MethodGet, orgRegister, actingPersonEtsi, "", nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid signers: status %d: %s", status, snippet(raw))
	}
	return parseSigners(raw)
}

func (c *client) AddSigner(ctx context.Context, orgRegister, actingPersonEtsi string, in AddSignerInput) (*SignersResult, error) {
	body := struct {
		SignerRegNo string `json:"signerRegNo"`
		Role        string `json:"role,omitempty"`
	}{SignerRegNo: strings.TrimSpace(in.SignerRegNo), Role: strings.TrimSpace(in.Role)}
	raw, status, err := c.signersReq(ctx, http.MethodPost, orgRegister, actingPersonEtsi, "", body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status == http.StatusNotFound {
		return nil, ErrSignerNotEnrolled
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid add signer: status %d: %s", status, snippet(raw))
	}
	return parseSignersResult(raw)
}

func (c *client) RemoveSigner(ctx context.Context, orgRegister, actingPersonEtsi, signerRegNo string) ([]Signer, error) {
	raw, status, err := c.signersReq(ctx, http.MethodDelete, orgRegister, actingPersonEtsi, strings.TrimSpace(signerRegNo), nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid remove signer: status %d: %s", status, snippet(raw))
	}
	return parseSigners(raw)
}

func (c *client) ResendSigner(ctx context.Context, orgRegister, actingPersonEtsi, signerRegNo string) (*SignersResult, error) {
	if strings.TrimSpace(orgRegister) == "" || strings.TrimSpace(actingPersonEtsi) == "" {
		return nil, errors.New("eid: empty orgRegister/actingPersonEtsi")
	}
	if strings.TrimSpace(signerRegNo) == "" {
		return nil, errors.New("eid: empty signerRegNo")
	}
	path := "/organization/signers/" + url.PathEscape(strings.TrimSpace(orgRegister)) + "/etsi/" +
		url.PathEscape(strings.TrimSpace(actingPersonEtsi)) + "/resend?signer=" + url.QueryEscape(strings.TrimSpace(signerRegNo))
	raw, status, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status == http.StatusNotFound {
		return nil, ErrSignerNotEnrolled
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid resend signer: status %d: %s", status, snippet(raw))
	}
	return parseSignersResult(raw)
}

// signersReq нь /organization/signers/{orgRegister}/etsi/{actingPersonEtsi} руу
// GET/POST/DELETE хүсэлт бүтээнэ. signer хоосон биш бол ?signer= query нэмнэ.
func (c *client) signersReq(ctx context.Context, method, orgRegister, actingPersonEtsi, signer string, body any) ([]byte, int, error) {
	if strings.TrimSpace(orgRegister) == "" || strings.TrimSpace(actingPersonEtsi) == "" {
		return nil, 0, errors.New("eid: empty orgRegister/actingPersonEtsi")
	}
	path := "/organization/signers/" + url.PathEscape(strings.TrimSpace(orgRegister)) + "/etsi/" + url.PathEscape(strings.TrimSpace(actingPersonEtsi))
	if signer != "" {
		path += "?signer=" + url.QueryEscape(signer)
	}
	switch method {
	case http.MethodPost:
		return c.post(ctx, path, body)
	case http.MethodDelete:
		return c.del(ctx, path)
	default:
		return c.get(ctx, path)
	}
}

// parseCertificate нь base64 DER сертификатыг задлан нээлттэй хэсгийг буцаана.
// Задлагдахгүй/хоосон бол nil (нэвтрэлтэд саад болохгүй).
func parseCertificate(b64 string) *Certificate {
	if strings.TrimSpace(b64) == "" {
		return nil
	}
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil
	}
	crt, err := x509.ParseCertificate(der)
	if err != nil {
		return nil
	}
	return &Certificate{
		Serial:    crt.SerialNumber.Text(16),
		NotBefore: crt.NotBefore,
		NotAfter:  crt.NotAfter,
		Issuer:    crt.Issuer.CommonName,
		KeyType:   keyTypeOf(crt),
	}
}

// keyTypeOf нь сертификатын нийтийн түлхүүрийн алгоритм + хэмжээг буцаана.
func keyTypeOf(crt *x509.Certificate) string {
	switch pub := crt.PublicKey.(type) {
	case *ecdsa.PublicKey:
		return "ECDSA " + pub.Curve.Params().Name
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", pub.N.BitLen())
	default:
		return crt.PublicKeyAlgorithm.String()
	}
}

// parseVC нь vc талбарыг задлана — anonymous нь string ("7270"), notification нь
// {"type":"alphaNumeric4","value":"0489"} object буцаадаг тул хоёуланг тэсвэрлэнэ.
func parseVC(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Value
	}
	return ""
}

// checkInitiateStatus нь initiate хариуны HTTP статусыг шалгана: 4xx = RP/оролтын
// алдаа (ErrInitiateRejected), бусад 3xx+ = дотоод алдаа.
func checkInitiateStatus(raw []byte, status int) error {
	if status >= 400 && status < 500 {
		return fmt.Errorf("%w: status %d: %s", ErrInitiateRejected, status, snippet(raw))
	}
	if status >= 300 {
		return fmt.Errorf("eid initiate: status %d: %s", status, snippet(raw))
	}
	return nil
}

func (c *client) post(ctx context.Context, path string, body any) ([]byte, int, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build request: %w", err)
	}
	c.setHeaders(req)
	return c.do(req)
}

func (c *client) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build request: %w", err)
	}
	c.setHeaders(req)
	return c.do(req)
}

func (c *client) del(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build request: %w", err)
	}
	c.setHeaders(req)
	return c.do(req)
}

func (c *client) put(ctx context.Context, path string, body any) ([]byte, int, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, fmt.Errorf("eid: build request: %w", err)
	}
	c.setHeaders(req)
	return c.do(req)
}

func (c *client) UpdateOrgNameLatin(ctx context.Context, orgRegister, actingPersonEtsi, nameLatin string) ([]Representation, error) {
	if strings.TrimSpace(orgRegister) == "" || strings.TrimSpace(actingPersonEtsi) == "" {
		return nil, errors.New("eid: empty orgRegister/actingPersonEtsi")
	}
	path := "/organization/name-latin/" + url.PathEscape(strings.TrimSpace(orgRegister)) + "/etsi/" + url.PathEscape(strings.TrimSpace(actingPersonEtsi))
	raw, status, err := c.put(ctx, path, map[string]string{"nameLatin": strings.TrimSpace(nameLatin)})
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden {
		return nil, ErrNotRepresentative
	}
	if status >= 300 {
		return nil, fmt.Errorf("eid update org name-latin: status %d: %s", status, snippet(raw))
	}
	return parseRepresentations(raw)
}

// setHeaders нь бүх хүсэлтэд RP Bearer secret болон JSON content-type-г тавина.
func (c *client) setHeaders(req *http.Request) {
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	req.Header.Set("Content-Type", "application/json")
}

func (c *client) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("eid: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	return raw, resp.StatusCode, nil
}

// randomHashB64 нь ACSP_V2 challenge болох 32 байт crypto/rand hash-г base64-std
// хэлбэрээр буцаана.
func randomHashB64() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
