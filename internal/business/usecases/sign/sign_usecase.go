// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package sign — PDF гарын үсэг (PAdES) eidmongolia.mn /v3-ээр. Хувь хүн.
//
// Урсгал: иргэн PDF оруулна → серверт hash тооцоод, eidmongolia /v3
// signature/notification/etsi-д digest илгээж утсанд PIN2 push явуулна → иргэн
// утсан дээрээ зөвшөөрнө (энэ нь хууль зүйн зөвшөөрөл) → сервер /v3 session-ийг
// poll хийж баталгаажуулна → татах үед СЕРВЕРИЙН Document-Signer гэрчилгээгээр
// PDF дотор PAdES гарын үсэг шигтгэж (digitorus/pdfsign), иргэний нэр/регистрийг
// гарын үсгийн талбарт тусгана. (Org-seal + TSA дараагийн үе шат.)
package sign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/digitorus/pdf"
	"github.com/digitorus/pdfsign/sign"

	"template/internal/apperror"
	"template/pkg/logger"
)

// cache нь caches.RedisCache-тэй тааруулсан нарийн интерфэйс (string round-trip;
// init↔poll↔download хооронд PDF төлөв). Утгыг JSON-string болгож хадгална.
type cache interface {
	Set(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string) (string, error)
}

// Config — /v3 RP тохиргоо.
type Config struct {
	V3BaseURL string // .well-known rp_api_base: https://eidmongolia.mn
	RPUUID    string // gerege RP UUID
	RPName    string // "gerege.mn"
	APISecret string // Bearer <api_secret> (хоосон бол илгээхгүй; registry унтраалттай үед хэрэггүй)

	// SignerCertPEM / SignerKeyPEM — серверийн БАЙНГЫН Document-Signer-ийн
	// гэрчилгээ ба ECDSA түлхүүр (PEM). Process restart/replica бүрт ИЖИЛ
	// байхын тулд гаднаас (secret) ачаална. Хоосон үед: production-д
	// NewUsecase алдаа буцаана (fail-closed), development-д dev self-signed.
	SignerCertPEM []byte
	SignerKeyPEM  []byte
	// IsProduction — хоосон signer тохиргоонд fail-closed эсэхийг тодорхойлно.
	IsProduction bool
}

type usecase struct {
	cache  cache
	cfg    Config
	client *http.Client
	signer signerIdentity // серверийн Document-Signer (process-д нэг удаа үүснэ)
}

// Usecase — нийтийн интерфэйс.
type Usecase interface {
	Init(ctx context.Context, regNo, fullName, filename string, pdf []byte) (InitResult, error)
	Poll(ctx context.Context, ownerRegNo, sessionID string) (string, error)
	Download(ctx context.Context, ownerRegNo, sessionID string) (DownloadResult, error)
}

type InitResult struct {
	SessionID        string `json:"session_id"`
	DocumentHash     string `json:"document_hash"`
	VerificationCode string `json:"verification_code"`
	Filename         string `json:"filename"`
}

type DownloadResult struct {
	PDF      []byte
	Filename string
}

// signState — Redis-д хадгалах session төлөв.
type signState struct {
	RegNo        string `json:"reg_no"`
	FullName     string `json:"full_name"`
	Filename     string `json:"filename"`
	PDFBase64    string `json:"pdf_b64"`
	DocHashB64   string `json:"doc_hash_b64"`
	V3SessionID  string `json:"v3_session_id"`
	State        string `json:"state"` // running | completed | failed | expired | rejected
	SignerName   string `json:"signer_name"`
	SignerSerial string `json:"signer_serial"`
	CompletedAt  string `json:"completed_at"`
}

const (
	statePrefix = "pdfsign:"
	maxPDFBytes = 25 << 20 // 25 MB
)

func (u *usecase) saveState(ctx context.Context, id string, st signState) error {
	b, _ := json.Marshal(st)
	return u.cache.Set(ctx, statePrefix+id, string(b))
}

func (u *usecase) loadState(ctx context.Context, id string) (signState, error) {
	var st signState
	s, err := u.cache.Get(ctx, statePrefix+id)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal([]byte(s), &st); err != nil {
		return st, err
	}
	return st, nil
}

// NewUsecase — серверийн Document-Signer-ийг ачаалж (production: тохируулсан
// байнгын PEM; development: dev self-signed fallback) usecase буцаана.
func NewUsecase(c cache, cfg Config) (Usecase, error) {
	id, err := resolveSigner(cfg)
	if err != nil {
		return nil, fmt.Errorf("sign: signer init: %w", err)
	}
	return &usecase{
		cache:  c,
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
		signer: id,
	}, nil
}

// resolveSigner — байнгын signer байвал түүнийг ачаална; эс бөгөөс production-д
// fail-closed (эфемер self-signed key нь reproducible/verifiable/revocable бус
// тул production-д хориглоно), development-д л dev self-signed-ийг зөвшөөрнө.
func resolveSigner(cfg Config) (signerIdentity, error) {
	if len(cfg.SignerCertPEM) > 0 && len(cfg.SignerKeyPEM) > 0 {
		return loadSignerPEM(cfg.SignerCertPEM, cfg.SignerKeyPEM)
	}
	if cfg.IsProduction {
		return signerIdentity{}, fmt.Errorf("production-д байнгын Document-Signer ЗААВАЛ: SIGN_SIGNER_CERT_FILE ба SIGN_SIGNER_KEY_FILE тохируул")
	}
	return newSelfSignedSigner()
}

// loadSignerPEM — PEM гэрчилгээ + ECDSA түлхүүрийг (SEC1 эсвэл PKCS8) задлана.
func loadSignerPEM(certPEM, keyPEM []byte) (signerIdentity, error) {
	cb, _ := pem.Decode(certPEM)
	if cb == nil || cb.Type != "CERTIFICATE" {
		return signerIdentity{}, fmt.Errorf("signer cert PEM буруу")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return signerIdentity{}, fmt.Errorf("signer cert parse: %w", err)
	}
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return signerIdentity{}, fmt.Errorf("signer key PEM буруу")
	}
	var key *ecdsa.PrivateKey
	switch kb.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(kb.Bytes)
	case "PRIVATE KEY":
		var k any
		if k, err = x509.ParsePKCS8PrivateKey(kb.Bytes); err == nil {
			ek, ok := k.(*ecdsa.PrivateKey)
			if !ok {
				return signerIdentity{}, fmt.Errorf("signer key ECDSA биш")
			}
			key = ek
		}
	default:
		return signerIdentity{}, fmt.Errorf("signer key PEM төрөл дэмжигдэхгүй: %q", kb.Type)
	}
	if err != nil {
		return signerIdentity{}, fmt.Errorf("signer key parse: %w", err)
	}
	return signerIdentity{key: key, cert: cert}, nil
}

// toEtsi — регистр/иргэний дугаараас ETSI semantic id (PNOMN-<...>).
func toEtsi(id string) string {
	v := strings.ToUpper(strings.TrimSpace(id))
	if strings.HasPrefix(v, "PNOMN-") || strings.HasPrefix(v, "NTRMN-") {
		return v
	}
	return "PNOMN-" + v
}

// regNoMatches — буцсан сертификатын serialNumber (ETSI: PNOMN-<РД> г.м.) дахь
// РД нь session эзний reg_no-той тохирч буйг шалгана. ETSI prefix болон үсгэн
// угтварын ялгаатай форматыг тойрохын тулд зөвхөн ОРОН ТООны цөмийг тулгана.
// certSerial-д орон алга бол (шалгах боломжгүй) үнэн буцаана — eID session-ийн
// өөрийн уялт хүчинтэй хэвээр тул хууль ёсны урсгалыг эвдэхгүй.
func regNoMatches(certSerial, regNo string) bool {
	digits := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	cd := digits(certSerial)
	if cd == "" {
		return true
	}
	return cd == digits(regNo)
}

// Init — PDF-ийн hash тооцоод /v3-д PIN2 sign session эхлүүлж, Redis-д хадгална.
func (u *usecase) Init(ctx context.Context, regNo, fullName, filename string, pdfBytes []byte) (InitResult, error) {
	if len(pdfBytes) == 0 || len(pdfBytes) > maxPDFBytes {
		return InitResult{}, apperror.BadRequest("PDF хэмжээ буруу (1 байт–25 MB)")
	}
	if strings.TrimSpace(regNo) == "" {
		return InitResult{}, apperror.Unauthorized("регистр тодорхойгүй")
	}
	sum := sha256.Sum256(pdfBytes)
	digestB64 := base64.StdEncoding.EncodeToString(sum[:])

	v3SessionID, vc, err := u.startV3Sign(ctx, toEtsi(regNo), digestB64, fullName)
	if err != nil {
		return InitResult{}, apperror.InternalCause(fmt.Errorf("v3 sign start: %w", err))
	}

	sessionID := randID()
	st := signState{
		RegNo: regNo, FullName: fullName, Filename: filename,
		PDFBase64:  base64.StdEncoding.EncodeToString(pdfBytes),
		DocHashB64: digestB64, V3SessionID: v3SessionID, State: "running",
	}
	if err := u.saveState(ctx, sessionID, st); err != nil {
		return InitResult{}, apperror.InternalCause(fmt.Errorf("sign state store: %w", err))
	}
	return InitResult{SessionID: sessionID, DocumentHash: digestB64, VerificationCode: vc, Filename: filename}, nil
}

// Poll — /v3 session-ийг шалгаж төлвийг шинэчилнэ.
func (u *usecase) Poll(ctx context.Context, ownerRegNo, sessionID string) (string, error) {
	st, err := u.loadState(ctx, sessionID)
	if err != nil {
		return "", apperror.NotFound("sign session олдсонгүй")
	}
	// Ownership: зөвхөн session-ийг эхлүүлсэн иргэн л хандана (IDOR-аас хамгаална).
	if st.RegNo != ownerRegNo {
		return "", apperror.NotFound("sign session олдсонгүй")
	}
	if st.State != "running" {
		return st.State, nil
	}
	res, err := u.pollV3(ctx, st.V3SessionID)
	if err != nil {
		return "running", nil // түр зуурын — дахин poll
	}
	switch {
	case res.State == "COMPLETE" && res.EndResult == "OK":
		// /v3 session нь toEtsi(st.RegNo)-оор эхэлсэн тул eID өөрөө буцах
		// сертификатыг тэр иргэнд уяна (login урсгал ч зөвхөн энэ уялтад
		// итгэдэг). Нэмэлт cross-check (буцсан cert-ийн serialNumber-ийн РД
		// session эзэнтэй тохирох) нь зарим eID cert-ийн serialNumber формат
		// (РД-ийн орон агуулаагүй) дээр ХУДАЛ бүтэлгүйтэл өгдөг тул блоклохгүй —
		// зөвхөн зөрөхөд анхааруулга бичнэ.
		if !regNoMatches(res.SubjectSerial, st.RegNo) {
			logger.WarnWithContext(ctx, "sign: cert serial РД-тэй тоон таарахгүй (non-blocking)", logger.Fields{
				"usecase": "sign", "method": "Poll",
				"cert_serial": res.SubjectSerial, "has_regno": st.RegNo != "",
			})
		}
		st.State = "completed"
		st.SignerName = res.SubjectName
		st.SignerSerial = res.SubjectSerial
		st.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	case res.State == "COMPLETE" && res.EndResult == "USER_REFUSED":
		st.State = "rejected"
	case res.State == "COMPLETE":
		logger.WarnWithContext(ctx, "sign: COMPLETE-ийн endResult OK/USER_REFUSED биш", logger.Fields{
			"usecase": "sign", "method": "Poll", "end_result": res.EndResult,
		})
		st.State = "failed"
	default:
		return "running", nil
	}
	_ = u.saveState(ctx, sessionID, st)
	return st.State, nil
}

// Download — completed session-ы PDF-д серверийн PAdES гарын үсэг шигтгэж буцаана.
func (u *usecase) Download(ctx context.Context, ownerRegNo, sessionID string) (DownloadResult, error) {
	st, err := u.loadState(ctx, sessionID)
	if err != nil {
		return DownloadResult{}, apperror.NotFound("sign session олдсонгүй")
	}
	// Ownership: зөвхөн session-ийг эхлүүлсэн иргэн л татаж авна (IDOR-аас хамгаална).
	if st.RegNo != ownerRegNo {
		return DownloadResult{}, apperror.NotFound("sign session олдсонгүй")
	}
	if st.State != "completed" {
		return DownloadResult{}, apperror.BadRequest("гарын үсэг дуусаагүй")
	}
	pdfBytes, err := base64.StdEncoding.DecodeString(st.PDFBase64)
	if err != nil {
		return DownloadResult{}, apperror.InternalCause(fmt.Errorf("pdf decode: %w", err))
	}
	signed, err := u.embedPAdES(pdfBytes, st)
	if err != nil {
		return DownloadResult{}, apperror.InternalCause(fmt.Errorf("pades embed: %w", err))
	}
	out := strings.TrimSuffix(st.Filename, ".pdf") + "-signed.pdf"
	return DownloadResult{PDF: signed, Filename: out}, nil
}

// embedPAdES — серверийн Document-Signer-ээр PDF-д гарын үсгийн dictionary шигтгэнэ.
// Гарын үсгийн нэр/шалтгаанд иргэний нэр/регистр + eID PIN2-ийн тэмдэглэгээ.
func (u *usecase) embedPAdES(pdfBytes []byte, st signState) ([]byte, error) {
	rdr, err := pdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		return nil, fmt.Errorf("pdf read: %w", err)
	}
	name := st.SignerName
	if name == "" {
		name = st.FullName
	}
	var out bytes.Buffer
	err = sign.Sign(bytes.NewReader(pdfBytes), &out, rdr, int64(len(pdfBytes)), sign.SignData{
		Signature: sign.SignDataSignature{
			Info: sign.SignDataSignatureInfo{
				Name:   name,
				Reason: "eID PIN2 гарын үсэг — РД " + st.RegNo,
				Date:   time.Now().Local(),
			},
			CertType:   sign.CertificationSignature,
			DocMDPPerm: sign.AllowFillingExistingFormFieldsAndSignaturesPerms,
		},
		Signer:          u.signer.key,
		Certificate:     u.signer.cert,
		DigestAlgorithm: crypto.SHA256,
	})
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ── серверийн Document-Signer (dev: self-signed P-256) ──

type signerIdentity struct {
	key  *ecdsa.PrivateKey
	cert *x509.Certificate
}

func newSelfSignedSigner() (signerIdentity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return signerIdentity{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "Gerege Document Signer", Country: []string{"MN"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(5, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return signerIdentity{}, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return signerIdentity{}, err
	}
	return signerIdentity{key: key, cert: cert}, nil
}

func randID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ── Gerege Platform /v3 RP client ──

// setRPAuth — RP secret тохируулсан бол Authorization: Bearer <api_secret> нэмнэ
// (.well-known: сервер SHA-256-аар RP-г resolve хийнэ). Хоосон бол алгасна —
// registry унтраалттай үед auth шаардлагагүй.
func (u *usecase) setRPAuth(req *http.Request) {
	if u.cfg.APISecret != "" {
		req.Header.Set("Authorization", "Bearer "+u.cfg.APISecret)
	}
}

func (u *usecase) startV3Sign(ctx context.Context, etsi, digestB64, displayName string) (sessionID, vc string, err error) {
	chal := make([]byte, 48)
	_, _ = rand.Read(chal)
	body := map[string]any{
		"relyingPartyUUID":  u.cfg.RPUUID,
		"relyingPartyName":  u.cfg.RPName,
		"certificateLevel":  "QUALIFIED",
		"signatureProtocol": "ACSP_V2",
		"digest":            digestB64,
		"hashType":          "SHA256",
		"interactions": []map[string]string{
			{"type": "displayTextAndPIN", "displayText60": "Gerege — баримтад гарын үсэг"},
		},
	}
	raw, _ := json.Marshal(body)
	reqURL := strings.TrimRight(u.cfg.V3BaseURL, "/") + "/v3/signature/notification/etsi/" + url.PathEscape(etsi)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	u.setRPAuth(req)
	res, err := u.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return "", "", fmt.Errorf("v3 %d: %s", res.StatusCode, string(b))
	}
	var r struct {
		SessionID string `json:"sessionID"`
		Vc        struct {
			Value string `json:"value"`
		} `json:"vc"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return "", "", err
	}
	return r.SessionID, r.Vc.Value, nil
}

type v3PollResult struct {
	State         string
	EndResult     string
	SubjectName   string
	SubjectSerial string
}

func (u *usecase) pollV3(ctx context.Context, v3SessionID string) (v3PollResult, error) {
	reqURL := strings.TrimRight(u.cfg.V3BaseURL, "/") + "/v3/session/" + url.PathEscape(v3SessionID) + "?timeoutMs=1000"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	u.setRPAuth(req)
	res, err := u.client.Do(req)
	if err != nil {
		return v3PollResult{}, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return v3PollResult{}, fmt.Errorf("v3 poll %d", res.StatusCode)
	}
	var r struct {
		State  string `json:"state"`
		Result struct {
			EndResult string `json:"endResult"`
		} `json:"result"`
		Cert struct {
			Value string `json:"value"`
		} `json:"cert"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return v3PollResult{}, err
	}
	out := v3PollResult{State: r.State, EndResult: r.Result.EndResult}
	if r.Cert.Value != "" {
		if der, e := base64.StdEncoding.DecodeString(r.Cert.Value); e == nil {
			if c, e2 := x509.ParseCertificate(der); e2 == nil {
				out.SubjectName = strings.TrimSpace(c.Subject.CommonName)
				out.SubjectSerial = c.Subject.SerialNumber
			}
		}
	}
	return out, nil
}
