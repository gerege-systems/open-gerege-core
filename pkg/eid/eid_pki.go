// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Иргэн өөрийн PKI самбарыг RP-ээр харах endpoint-уудын client (eid-platform-mn
// docs/EID_PERSON_PKI.md). Эдгээр нь PII тул зөвхөн админаас PKI_READ эрх
// олгосон RP-д нээгддэг — эрхгүй бол 403 → ErrPKINotPermitted буцна.
//
//	GET /v3/certificates/etsi/{personEtsi}   — гэрчилгээний жагсаалт + статусын тоо
//	GET /v3/devices/etsi/{personEtsi}         — холбоотой төхөөрөмжүүд
//	GET /v3/rp/activity/etsi/{personEtsi}     — RP-scoped auth/sign түүх + тоо
//	GET /v3/person/summary/etsi/{personEtsi}  — dashboard-ын нэгдсэн тоо
package eid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrPKINotPermitted нь RP-д PKI_READ эрх олгогдоогүй (403) үед буцна. Дуудагч
// үүнийг "эрх хүлээгдэж байна" төлөв болгон харуулж болно (алдаа биш).
var ErrPKINotPermitted = errors.New("eid: RP lacks PKI_READ permission")

// ErrNotRepresentative нь AddRepresentation-д иргэн тухайн байгууллагыг төлөөлөх
// эрхгүй (РД нь affiliate жагсаалтад алга) үед 403-аар буцна. Дуудагч үүнийг
// "энэ байгууллагыг төлөөлөх эрхгүй" (Forbidden) болгон харуулна.
var ErrNotRepresentative = errors.New("eid: not authorized to represent this organization")

// ErrSignerNotEnrolled нь AddSigner-д нэмэх гэсэн иргэн eID-д бүртгэлгүй (РД
// олдсонгүй, 404) үед буцна. Гарын үсэг зурахад eID шаардлагатай.
var ErrSignerNotEnrolled = errors.New("eid: signer is not enrolled in eID")

// CertCounts нь гэрчилгээний статусын тоолол.
type CertCounts struct {
	Valid     int `json:"valid"`
	Revoked   int `json:"revoked"`
	Expired   int `json:"expired"`
	Suspended int `json:"suspended"`
	Total     int `json:"total"`
}

// PersonCertItem нь иргэний нэг гэрчилгээ.
type PersonCertItem struct {
	DocumentNumber   string    `json:"documentNumber"`
	Type             string    `json:"type"` // AUTH | SIGN | SEAL
	SerialNumber     string    `json:"serialNumber"`
	CertificateLevel string    `json:"certificateLevel"`
	Status           string    `json:"status"` // VALID | REVOKED | EXPIRED | SUSPENDED
	NotBefore        time.Time `json:"notBefore"`
	NotAfter         time.Time `json:"notAfter"`
	IssuerDn         string    `json:"issuerDn"`
}

// PersonCertificates нь GET /v3/certificates/etsi/{personEtsi}-ийн хариу.
type PersonCertificates struct {
	Counts       CertCounts       `json:"counts"`
	Certificates []PersonCertItem `json:"certificates"`
}

// ActivityCounts нь flow тус бүрийн тоолол.
type ActivityCounts struct {
	Authentication int `json:"authentication"`
	Signature      int `json:"signature"`
}

// PersonActivityItem нь RP-scoped session түүхийн нэг бичлэг. Extra нь activity
// service-ийн буцаадаг нэмэлт (танихгүй) талбаруудыг задлан хадгална — upstream
// өргөжихөд UI ямар ч талбарыг харуулж чадна ("бүгдийг харуул").
type PersonActivityItem struct {
	SessionID string         `json:"sessionId"`
	Flow      string         `json:"flow"` // AUTHENTICATION | SIGNATURE
	Outcome   string         `json:"outcome"`
	DocText   string         `json:"docText"`
	Timestamp time.Time      `json:"timestamp"`
	Extra     map[string]any `json:"-"` // танигдсанаас бусад бүх талбар (raw)
}

// activityKnownKeys нь дээрх typed талбаруудын JSON нэрс — Extra-д давхардуулахгүй.
var activityKnownKeys = map[string]struct{}{
	"sessionId": {}, "flow": {}, "outcome": {}, "docText": {}, "timestamp": {},
}

// UnmarshalJSON нь typed талбаруудыг бөглөөд, үлдсэн бүх түлхүүрийг Extra-д хийнэ.
func (i *PersonActivityItem) UnmarshalJSON(b []byte) error {
	type alias PersonActivityItem // рекурс сэргийлэх (Extra нь json:"-")
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*i = PersonActivityItem(a)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		if _, known := activityKnownKeys[k]; known {
			continue
		}
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			continue // задлагдахгүй талбарыг алгасна
		}
		if i.Extra == nil {
			i.Extra = map[string]any{}
		}
		i.Extra[k] = val
	}
	return nil
}

// PersonActivity нь GET /v3/rp/activity/etsi/{personEtsi}-ийн хариу (RP-scoped).
type PersonActivity struct {
	Counts   ActivityCounts       `json:"counts"`
	Sessions []PersonActivityItem `json:"sessions"`
	Total    int                  `json:"total"`
}

// PersonDeviceItem нь холбоотой нэг төхөөрөмж. Extra нь upstream-ийн буцаадаг
// нэмэлт (танихгүй) талбаруудыг задлан хадгална ("бүгдийг харуул").
type PersonDeviceItem struct {
	DocumentNumber string         `json:"documentNumber"`
	Platform       string         `json:"platform"` // APNS | FCM
	EnrolledAt     time.Time      `json:"enrolledAt"`
	Active         bool           `json:"active"`
	DeactivatedAt  *time.Time     `json:"deactivatedAt"`
	Extra          map[string]any `json:"-"` // танигдсанаас бусад бүх талбар (raw)
}

// deviceKnownKeys нь дээрх typed талбаруудын JSON нэрс — Extra-д давхардуулахгүй.
var deviceKnownKeys = map[string]struct{}{
	"documentNumber": {}, "platform": {}, "enrolledAt": {}, "active": {}, "deactivatedAt": {},
}

// UnmarshalJSON нь typed талбаруудыг бөглөөд, үлдсэн бүх түлхүүрийг Extra-д хийнэ.
func (i *PersonDeviceItem) UnmarshalJSON(b []byte) error {
	type alias PersonDeviceItem // рекурс сэргийлэх (Extra нь json:"-")
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*i = PersonDeviceItem(a)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		if _, known := deviceKnownKeys[k]; known {
			continue
		}
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			continue
		}
		if i.Extra == nil {
			i.Extra = map[string]any{}
		}
		i.Extra[k] = val
	}
	return nil
}

// PersonDevices нь GET /v3/devices/etsi/{personEtsi}-ийн хариу.
type PersonDevices struct {
	Devices     []PersonDeviceItem `json:"devices"`
	ActiveCount int                `json:"activeCount"`
	Total       int                `json:"total"`
}

// PersonSummary нь GET /v3/person/summary/etsi/{personEtsi} — dashboard-ын
// нэгдсэн тоо (нэг дуудлагаар гэрчилгээ/activity/төхөөрөмж/байгууллага).
type PersonSummary struct {
	GivenName           string         `json:"givenName"`
	Surname             string         `json:"surname"`
	Certificates        CertCounts     `json:"certificates"`
	Activity            ActivityCounts `json:"activity"`
	DevicesActive       int            `json:"devicesActive"`
	DevicesTotal        int            `json:"devicesTotal"`
	RepresentationCount int            `json:"representationCount"`
}

// getPKI нь PKI endpoint-ыг дуудаж хариуг v рүү задлана. 403 бол
// ErrPKINotPermitted, 404 бол ErrInitiateRejected-гүйгээр зүгээр хоосон
// (дуудагч nil шалгана).
func getPKI[T any](ctx context.Context, c *client, path string, v *T) error {
	raw, status, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	if status == http.StatusForbidden {
		return ErrPKINotPermitted
	}
	if status == http.StatusNotFound {
		return nil // хүн/өгөгдөл олдсонгүй — тэг утгатай v үлдэнэ
	}
	if status >= 300 {
		return fmt.Errorf("eid pki: status %d: %s", status, snippet(raw))
	}
	if jErr := json.Unmarshal(raw, v); jErr != nil {
		return fmt.Errorf("eid pki: invalid response: %s", snippet(raw))
	}
	return nil
}

func etsiPath(prefix, personEtsi string) (string, error) {
	p := strings.TrimSpace(personEtsi)
	if p == "" {
		return "", errors.New("eid: empty personEtsi")
	}
	return prefix + url.PathEscape(p), nil
}

func (c *client) PersonSummary(ctx context.Context, personEtsi string) (*PersonSummary, error) {
	path, err := etsiPath("/person/summary/etsi/", personEtsi)
	if err != nil {
		return nil, err
	}
	var out PersonSummary
	if err := getPKI(ctx, c, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *client) PersonCertificates(ctx context.Context, personEtsi string) (*PersonCertificates, error) {
	path, err := etsiPath("/certificates/etsi/", personEtsi)
	if err != nil {
		return nil, err
	}
	var out PersonCertificates
	if err := getPKI(ctx, c, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *client) PersonDevices(ctx context.Context, personEtsi string) (*PersonDevices, error) {
	path, err := etsiPath("/devices/etsi/", personEtsi)
	if err != nil {
		return nil, err
	}
	var out PersonDevices
	if err := getPKI(ctx, c, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *client) PersonActivity(ctx context.Context, personEtsi string, limit, offset int) (*PersonActivity, error) {
	base, err := etsiPath("/rp/activity/etsi/", personEtsi)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := base + "?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset)
	var out PersonActivity
	if err := getPKI(ctx, c, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
