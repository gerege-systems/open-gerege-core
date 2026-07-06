// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/pkg/eid"
)

// OrgRepresentationResponse нь иргэний төлөөлдөг нэг байгууллага.
type OrgRepresentationResponse struct {
	OrgEtsi     string     `json:"org_etsi"`
	OrgRegister string     `json:"org_register"`
	OrgName     string     `json:"org_name"`
	OrgNameEn   string     `json:"org_name_en,omitempty"`
	Role        string     `json:"role,omitempty"`
	RightType   string     `json:"right_type,omitempty"`
	ValidFrom   *time.Time `json:"valid_from,omitempty"`
	ValidTo     *time.Time `json:"valid_to,omitempty"`
}

// FromEIDRepresentations нь eID representation-уудыг DTO жагсаалт руу буулгана.
func FromEIDRepresentations(reps []eid.Representation) []OrgRepresentationResponse {
	out := make([]OrgRepresentationResponse, 0, len(reps))
	for _, r := range reps {
		out = append(out, OrgRepresentationResponse{
			OrgEtsi:     r.OrgEtsi,
			OrgRegister: r.OrgRegister,
			OrgName:     r.OrgName,
			OrgNameEn:   r.OrgNameEn,
			Role:        r.Role,
			RightType:   r.RightType,
			ValidFrom:   r.ValidFrom,
			ValidTo:     r.ValidTo,
		})
	}
	return out
}

// OrgSignerResponse нь байгууллагыг төлөөлж / гарын үсэг зурж чадах нэг иргэн.
type OrgSignerResponse struct {
	PersonEtsi string `json:"person_etsi"`
	RegNo      string `json:"reg_no,omitempty"`
	Name       string `json:"name,omitempty"`
	NameEn     string `json:"name_en,omitempty"`
	Role       string `json:"role,omitempty"`
	RightType  string `json:"right_type"`
	Status     string `json:"status"` // ACTIVE | PENDING (sign-push баталгаажуулалт)
	Source     string `json:"source"`
	Self       bool   `json:"self"`
}

// OrgPendingConfirmationResponse нь MANAGER нэмэхэд тэр хүн рүү илгээгдсэн sign-push
// баталгаажуулалтын мэдээлэл (клиент "хүсэлт илгээгдлээ" гэж харуулна).
type OrgPendingConfirmationResponse struct {
	SignerEtsi  string `json:"signer_etsi"`
	SignerRegNo string `json:"signer_reg_no,omitempty"`
	SessionID   string `json:"session_id"`
}

// OrgSignersResultResponse нь POST (нэмэх)-ийн хариу — жагсаалт + хүлээгдэж буй баталгаажуулалт.
type OrgSignersResultResponse struct {
	Signers             []OrgSignerResponse             `json:"signers"`
	PendingConfirmation *OrgPendingConfirmationResponse `json:"pending_confirmation,omitempty"`
}

// FromEIDSigners нь eID signer-уудыг DTO жагсаалт руу буулгана.
func FromEIDSigners(signers []eid.Signer) []OrgSignerResponse {
	out := make([]OrgSignerResponse, 0, len(signers))
	for _, s := range signers {
		out = append(out, OrgSignerResponse{
			PersonEtsi: s.PersonEtsi,
			RegNo:      s.RegNo,
			Name:       s.Name,
			NameEn:     s.NameEn,
			Role:       s.Role,
			RightType:  s.RightType,
			Status:     s.Status,
			Source:     s.Source,
			Self:       s.Self,
		})
	}
	return out
}

// FromEIDSignersResult нь AddSigner-ийн үр дүнг (жагсаалт + pending) DTO руу буулгана.
func FromEIDSignersResult(res *eid.SignersResult) OrgSignersResultResponse {
	if res == nil {
		return OrgSignersResultResponse{Signers: []OrgSignerResponse{}}
	}
	out := OrgSignersResultResponse{Signers: FromEIDSigners(res.Signers)}
	if pc := res.PendingConfirmation; pc != nil {
		out.PendingConfirmation = &OrgPendingConfirmationResponse{
			SignerEtsi: pc.SignerEtsi, SignerRegNo: pc.SignerRegNo, SessionID: pc.SessionID,
		}
	}
	return out
}

// ── PKI самбар (snake_case DTO-нууд) ──

type EIDCertCounts struct {
	Valid     int `json:"valid"`
	Revoked   int `json:"revoked"`
	Expired   int `json:"expired"`
	Suspended int `json:"suspended"`
	Total     int `json:"total"`
}

type EIDActivityCounts struct {
	Authentication int `json:"authentication"`
	Signature      int `json:"signature"`
}

type EIDPersonCert struct {
	DocumentNumber   string     `json:"document_number"`
	Type             string     `json:"type"`
	SerialNumber     string     `json:"serial_number"`
	CertificateLevel string     `json:"certificate_level"`
	Status           string     `json:"status"`
	NotBefore        *time.Time `json:"not_before,omitempty"`
	NotAfter         *time.Time `json:"not_after,omitempty"`
	IssuerDn         string     `json:"issuer_dn,omitempty"`
}

type EIDCertificatesResponse struct {
	Counts       EIDCertCounts   `json:"counts"`
	Certificates []EIDPersonCert `json:"certificates"`
}

type EIDPersonDevice struct {
	DocumentNumber string     `json:"document_number"`
	Platform       string     `json:"platform,omitempty"`
	EnrolledAt     *time.Time `json:"enrolled_at,omitempty"`
	Active         bool       `json:"active"`
	DeactivatedAt  *time.Time `json:"deactivated_at,omitempty"`
}

type EIDDevicesResponse struct {
	Devices     []EIDPersonDevice `json:"devices"`
	ActiveCount int               `json:"active_count"`
	Total       int               `json:"total"`
}

type EIDActivityItem struct {
	SessionID string     `json:"session_id,omitempty"`
	Flow      string     `json:"flow"`
	Outcome   string     `json:"outcome"`
	DocText   string     `json:"doc_text,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

type EIDActivityResponse struct {
	Counts   EIDActivityCounts `json:"counts"`
	Sessions []EIDActivityItem `json:"sessions"`
	Total    int               `json:"total"`
}

type EIDSummaryResponse struct {
	GivenName           string            `json:"given_name,omitempty"`
	Surname             string            `json:"surname,omitempty"`
	Certificates        EIDCertCounts     `json:"certificates"`
	Activity            EIDActivityCounts `json:"activity"`
	DevicesActive       int               `json:"devices_active"`
	DevicesTotal        int               `json:"devices_total"`
	RepresentationCount int               `json:"representation_count"`
}

func certCounts(c eid.CertCounts) EIDCertCounts {
	return EIDCertCounts{Valid: c.Valid, Revoked: c.Revoked, Expired: c.Expired, Suspended: c.Suspended, Total: c.Total}
}

func tptr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// FromEIDCertificates нь eID гэрчилгээний хариуг DTO руу буулгана (nil → хоосон).
func FromEIDCertificates(c *eid.PersonCertificates) EIDCertificatesResponse {
	if c == nil {
		return EIDCertificatesResponse{Certificates: []EIDPersonCert{}}
	}
	certs := make([]EIDPersonCert, 0, len(c.Certificates))
	for _, x := range c.Certificates {
		certs = append(certs, EIDPersonCert{
			DocumentNumber: x.DocumentNumber, Type: x.Type, SerialNumber: x.SerialNumber,
			CertificateLevel: x.CertificateLevel, Status: x.Status,
			NotBefore: tptr(x.NotBefore), NotAfter: tptr(x.NotAfter), IssuerDn: x.IssuerDn,
		})
	}
	return EIDCertificatesResponse{Counts: certCounts(c.Counts), Certificates: certs}
}

// FromEIDDevices нь төхөөрөмжийн хариуг DTO руу буулгана (nil → хоосон).
func FromEIDDevices(d *eid.PersonDevices) EIDDevicesResponse {
	if d == nil {
		return EIDDevicesResponse{Devices: []EIDPersonDevice{}}
	}
	devs := make([]EIDPersonDevice, 0, len(d.Devices))
	for _, x := range d.Devices {
		devs = append(devs, EIDPersonDevice{
			DocumentNumber: x.DocumentNumber, Platform: x.Platform,
			EnrolledAt: tptr(x.EnrolledAt), Active: x.Active, DeactivatedAt: x.DeactivatedAt,
		})
	}
	return EIDDevicesResponse{Devices: devs, ActiveCount: d.ActiveCount, Total: d.Total}
}

// FromEIDActivity нь activity хариуг DTO руу буулгана (nil → хоосон).
func FromEIDActivity(a *eid.PersonActivity) EIDActivityResponse {
	if a == nil {
		return EIDActivityResponse{Sessions: []EIDActivityItem{}}
	}
	items := make([]EIDActivityItem, 0, len(a.Sessions))
	for _, x := range a.Sessions {
		items = append(items, EIDActivityItem{
			SessionID: x.SessionID, Flow: x.Flow, Outcome: x.Outcome, DocText: x.DocText, Timestamp: tptr(x.Timestamp),
		})
	}
	return EIDActivityResponse{
		Counts:   EIDActivityCounts{Authentication: a.Counts.Authentication, Signature: a.Counts.Signature},
		Sessions: items, Total: a.Total,
	}
}

// FromEIDSummary нь нэгдсэн хариуг DTO руу буулгана (nil → хоосон).
func FromEIDSummary(s *eid.PersonSummary) EIDSummaryResponse {
	if s == nil {
		return EIDSummaryResponse{}
	}
	return EIDSummaryResponse{
		GivenName: s.GivenName, Surname: s.Surname,
		Certificates:        certCounts(s.Certificates),
		Activity:            EIDActivityCounts{Authentication: s.Activity.Authentication, Signature: s.Activity.Signature},
		DevicesActive:       s.DevicesActive,
		DevicesTotal:        s.DevicesTotal,
		RepresentationCount: s.RepresentationCount,
	}
}
