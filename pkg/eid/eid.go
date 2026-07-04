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
	RightType   string // SOLE | JOINT
	ValidFrom   *time.Time
	ValidTo     *time.Time // nil = хугацаагүй
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
	Initiate(ctx context.Context, nationalID, displayText, nonce string) (*StartResult, error)
	// Session нь session-ийн төлвийг long-poll-оор асууна (timeoutMs хүртэл).
	Session(ctx context.Context, sessionID string, timeoutMs int) (*SessionResult, error)
	// Representations нь тухайн хүн (personEtsi = PNOMN-<civil_id>)-ий төлөөлж
	// чадах идэвхтэй байгууллагуудыг буцаана. Иргэн байгууллага төлөөлдөггүй
	// бол хоосон slice.
	Representations(ctx context.Context, personEtsi string) ([]Representation, error)

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
}

func (c *client) newAuthBody(displayText string) (authInitiateBody, error) {
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
		RelyingPartyUUID:  c.rpUUID,
		RelyingPartyName:  c.rpName,
		CertificateLevel:  c.certLevel,
		SignatureProtocol: "ACSP_V2",
		RPChallenge:       challenge,
		Interactions:      []interaction{{Type: "displayTextAndPIN", DisplayText60: dt}},
	}, nil
}

func (c *client) QRInitiate(ctx context.Context, displayText, _, _ string) (*StartResult, error) {
	body, err := c.newAuthBody(displayText)
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

func (c *client) Initiate(ctx context.Context, nationalID, displayText, _ string) (*StartResult, error) {
	body, err := c.newAuthBody(displayText)
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
