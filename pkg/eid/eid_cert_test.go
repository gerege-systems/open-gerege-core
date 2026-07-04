// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// login COMPLETE-ийн cert.value (base64 DER)-ээс сертификатыг задлах тест.
// Тест өөрөө self-signed ECDSA сертификат үүсгэж, session COMPLETE хариунд
// оруулж, client нь серийн дугаар / хүчинтэй хугацаа / issuer / түлхүүрийн
// төрөл / documentNumber-г зөв гаргаж авахыг шалгана. Cert байхгүй эсвэл хог
// байвал нэвтрэлт зогсохгүй (Certificate=nil) гэдгийг ч батална.
package eid

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// makeTestCertB64 нь self-signed ECDSA P-256 сертификат үүсгэж base64-std DER
// буцаана (+ хүлээгдэх notAfter).
func makeTestCertB64(t *testing.T) (string, time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	notAfter := time.Now().Add(365 * 24 * time.Hour).UTC().Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(0x1a2b3c),
		Subject:      pkix.Name{CommonName: "PNOMN-УБ99887766"},
		Issuer:       pkix.Name{CommonName: "eID Mongolia Qualified CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der), notAfter
}

func sessionServer(t *testing.T, body string) Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, testUUID, testName, testSecret, "ADVANCED")
}

func TestSessionParsesCertificate(t *testing.T) {
	certB64, notAfter := makeTestCertB64(t)
	body := fmt.Sprintf(`{
		"state":"COMPLETE",
		"result":{"endResult":"OK","documentNumber":"DOC-abc-123"},
		"cert":{"value":%q,"certificateLevel":"QUALIFIED"},
		"person":{"givenName":"Бат","surname":"Дорж","civilId":"УБ99887766","regNo":"1234567"}
	}`, certB64)

	res, err := sessionServer(t, body).Session(context.Background(), "s1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateComplete || res.Identity == nil {
		t.Fatalf("state=%s identity=%v", res.State, res.Identity)
	}
	if res.Identity.DocumentNumber != "DOC-abc-123" {
		t.Errorf("documentNumber = %q", res.Identity.DocumentNumber)
	}
	c := res.Identity.Certificate
	if c == nil {
		t.Fatal("Certificate задлагдсангүй")
	}
	if c.Serial != "1a2b3c" {
		t.Errorf("serial = %q, want 1a2b3c", c.Serial)
	}
	if !c.NotAfter.Equal(notAfter) {
		t.Errorf("notAfter = %v, want %v", c.NotAfter, notAfter)
	}
	// Self-signed тул issuer CN нь subject CN-тэй тэнцүү.
	if c.Issuer != "PNOMN-УБ99887766" {
		t.Errorf("issuer = %q", c.Issuer)
	}
	if c.KeyType != "ECDSA P-256" {
		t.Errorf("keyType = %q, want ECDSA P-256", c.KeyType)
	}
}

func TestSessionCompleteWithoutCertStillLogsIn(t *testing.T) {
	// cert блок огт байхгүй — Certificate=nil, гэхдээ нэвтрэлт амжилттай.
	body := `{"state":"COMPLETE","result":{"endResult":"OK"},"person":{"civilId":"УБ1","regNo":"1"}}`
	res, err := sessionServer(t, body).Session(context.Background(), "s1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateComplete || res.Identity.Certificate != nil {
		t.Errorf("cert-гүй COMPLETE: state=%s cert=%v", res.State, res.Identity.Certificate)
	}
}

func TestParseCertificateGarbage(t *testing.T) {
	if parseCertificate("") != nil {
		t.Error("хоосон → nil байх ёстой")
	}
	if parseCertificate("not-base64!!!") != nil {
		t.Error("буруу base64 → nil байх ёстой")
	}
	if parseCertificate(base64.StdEncoding.EncodeToString([]byte("not a der cert"))) != nil {
		t.Error("буруу DER → nil байх ёстой")
	}
}
