// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"encoding/json"
	"time"

	"template/internal/business/domain"
	registryuc "template/internal/business/usecases/registry"
)

// Ring System · R1 — Үйлчилгээний нэгдсэн регистрийн хариу DTO-ууд.

// ── Паспорт ─────────────────────────────────────────────────────────────────

type RegistryEvidenceLinkResponse struct {
	EvidenceID  string `json:"evidence_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	FromCitizen bool   `json:"from_citizen"`
	InKHUR      bool   `json:"in_khur"`
	// OnceOnlyViolation нь энэ мөр өөрөө зөрчил эсэх (иргэнээс шаардаж буй
	// АТАЛ ХУР-д байгаа) — UI-д тусад нь тооцоолох шаардлагагүй.
	OnceOnlyViolation bool   `json:"once_only_violation"`
	Note              string `json:"note"`
}

type RegistryServiceResponse struct {
	ID             string     `json:"id"`
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	NameEN         string     `json:"name_en"`
	Description    string     `json:"description"`
	Authority      string     `json:"authority"`
	AuthorityOrgID *string    `json:"authority_org_id"`
	LegalBasis     string     `json:"legal_basis"`
	TargetGroup    string     `json:"target_group"`
	Output         string     `json:"output"`
	Channels       []string   `json:"channels"`
	Fee            int        `json:"fee"`
	MaxDays        int        `json:"max_days"`
	StepsCount     int        `json:"steps_count"`
	AnnualVolume   int        `json:"annual_volume"`
	Proactivity    string     `json:"proactivity"`
	Status         string     `json:"status"`
	LifeEventID    *string    `json:"life_event_id"`
	Version        int        `json:"version"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`

	// Зөвхөн дэлгэрэнгүй уншилтад дүүргэгдэнэ.
	Evidences []RegistryEvidenceLinkResponse `json:"evidences,omitempty"`
}

func FromRegistryService(s domain.RegistryService) RegistryServiceResponse {
	out := RegistryServiceResponse{
		ID: s.ID, Code: s.Code, Name: s.Name, NameEN: s.NameEN, Description: s.Description,
		Authority: s.Authority, AuthorityOrgID: s.AuthorityOrgID, LegalBasis: s.LegalBasis,
		TargetGroup: s.TargetGroup, Output: s.Output, Channels: s.Channels, Fee: s.Fee,
		MaxDays: s.MaxDays, StepsCount: s.StepsCount, AnnualVolume: s.AnnualVolume,
		Proactivity: s.Proactivity, Status: s.Status, LifeEventID: s.LifeEventID,
		Version: s.Version, PublishedAt: s.PublishedAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if out.Channels == nil {
		out.Channels = []string{}
	}
	if len(s.Evidences) > 0 {
		out.Evidences = make([]RegistryEvidenceLinkResponse, 0, len(s.Evidences))
		for _, e := range s.Evidences {
			out.Evidences = append(out.Evidences, RegistryEvidenceLinkResponse{
				EvidenceID: e.EvidenceID, Code: e.Code, Name: e.Name,
				Required: e.Required, FromCitizen: e.FromCitizen, InKHUR: e.InKHUR,
				OnceOnlyViolation: e.FromCitizen && e.InKHUR,
				Note:              e.Note,
			})
		}
	}
	return out
}

func ToRegistryServiceList(list []domain.RegistryService) []RegistryServiceResponse {
	out := make([]RegistryServiceResponse, 0, len(list))
	for _, s := range list {
		out = append(out, FromRegistryService(s))
	}
	return out
}

// ── Хувилбар ────────────────────────────────────────────────────────────────

type RegistryVersionResponse struct {
	ID             string          `json:"id"`
	ServiceID      string          `json:"service_id"`
	Version        int             `json:"version"`
	Snapshot       json.RawMessage `json:"snapshot,omitempty"`
	ChangeNote     string          `json:"change_note"`
	IsBaseline     bool            `json:"is_baseline"`
	StepsCount     int             `json:"steps_count"`
	DocumentsCount int             `json:"documents_count"`
	MaxDays        int             `json:"max_days"`
	Fee            int             `json:"fee"`
	// Delta* нь baseline-тай харьцуулсан ялгаа — СӨРӨГ утга нь сайжралт
	// (алхам/баримт/хугацаа буурсан).
	DeltaSteps     int       `json:"delta_steps"`
	DeltaDocuments int       `json:"delta_documents"`
	DeltaDays      int       `json:"delta_days"`
	DeltaFee       int       `json:"delta_fee"`
	PublishedAt    time.Time `json:"published_at"`
}

func FromRegistryVersion(v domain.RegistryServiceVersion) RegistryVersionResponse {
	return RegistryVersionResponse{
		ID: v.ID, ServiceID: v.ServiceID, Version: v.Version, Snapshot: json.RawMessage(v.Snapshot),
		ChangeNote: v.ChangeNote, IsBaseline: v.IsBaseline, StepsCount: v.StepsCount,
		DocumentsCount: v.DocumentsCount, MaxDays: v.MaxDays, Fee: v.Fee,
		DeltaSteps: v.DeltaSteps, DeltaDocuments: v.DeltaDocuments, DeltaDays: v.DeltaDays,
		DeltaFee: v.DeltaFee, PublishedAt: v.PublishedAt,
	}
}

func ToRegistryVersionList(list []domain.RegistryServiceVersion) []RegistryVersionResponse {
	out := make([]RegistryVersionResponse, 0, len(list))
	for _, v := range list {
		out = append(out, FromRegistryVersion(v))
	}
	return out
}

// ── Нотолгооны каталог ──────────────────────────────────────────────────────

type RegistryEvidenceResponse struct {
	ID              string     `json:"id"`
	Code            string     `json:"code"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	HolderAgency    string     `json:"holder_agency"`
	SourceSystem    string     `json:"source_system"`
	InKHUR          bool       `json:"in_khur"`
	KHURServiceCode string     `json:"khur_service_code"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
}

func FromRegistryEvidence(e domain.RegistryEvidence) RegistryEvidenceResponse {
	return RegistryEvidenceResponse{
		ID: e.ID, Code: e.Code, Name: e.Name, Description: e.Description,
		HolderAgency: e.HolderAgency, SourceSystem: e.SourceSystem,
		InKHUR: e.InKHUR, KHURServiceCode: e.KHURServiceCode,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func ToRegistryEvidenceList(list []domain.RegistryEvidence) []RegistryEvidenceResponse {
	out := make([]RegistryEvidenceResponse, 0, len(list))
	for _, e := range list {
		out = append(out, FromRegistryEvidence(e))
	}
	return out
}

// ── Амьдралын үйл явдал ─────────────────────────────────────────────────────

type RegistryLifeEventResponse struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	LeadAgency  string    `json:"lead_agency"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

func FromRegistryLifeEvent(l domain.RegistryLifeEvent) RegistryLifeEventResponse {
	return RegistryLifeEventResponse{
		ID: l.ID, Code: l.Code, Name: l.Name, Kind: l.Kind, Description: l.Description,
		LeadAgency: l.LeadAgency, SortOrder: l.SortOrder, CreatedAt: l.CreatedAt,
	}
}

func ToRegistryLifeEventList(list []domain.RegistryLifeEvent) []RegistryLifeEventResponse {
	out := make([]RegistryLifeEventResponse, 0, len(list))
	for _, l := range list {
		out = append(out, FromRegistryLifeEvent(l))
	}
	return out
}

// ── Once-only ───────────────────────────────────────────────────────────────

type RegistryOnceOnlyViolationResponse struct {
	ServiceID       string `json:"service_id"`
	ServiceCode     string `json:"service_code"`
	ServiceName     string `json:"service_name"`
	Authority       string `json:"authority"`
	ServiceStatus   string `json:"service_status"`
	EvidenceID      string `json:"evidence_id"`
	EvidenceCode    string `json:"evidence_code"`
	EvidenceName    string `json:"evidence_name"`
	HolderAgency    string `json:"holder_agency"`
	KHURServiceCode string `json:"khur_service_code"`
	AnnualVolume    int    `json:"annual_volume"`
}

func ToRegistryOnceOnlyList(list []domain.RegistryOnceOnlyViolation) []RegistryOnceOnlyViolationResponse {
	out := make([]RegistryOnceOnlyViolationResponse, 0, len(list))
	for _, v := range list {
		out = append(out, RegistryOnceOnlyViolationResponse{
			ServiceID: v.ServiceID, ServiceCode: v.ServiceCode, ServiceName: v.ServiceName,
			Authority: v.Authority, ServiceStatus: v.ServiceStatus, EvidenceID: v.EvidenceID,
			EvidenceCode: v.EvidenceCode, EvidenceName: v.EvidenceName,
			HolderAgency: v.HolderAgency, KHURServiceCode: v.KHURServiceCode,
			AnnualVolume: v.AnnualVolume,
		})
	}
	return out
}

// RegistryOnceOnlyReportResponse нь нэг үйлчилгээний шалгалтын дүн.
type RegistryOnceOnlyReportResponse struct {
	ServiceID        string                         `json:"service_id"`
	ServiceCode      string                         `json:"service_code"`
	ServiceName      string                         `json:"service_name"`
	CitizenDocuments int                            `json:"citizen_documents"`
	Violations       []RegistryEvidenceLinkResponse `json:"violations"`
	Compliant        bool                           `json:"compliant"`
	// EligibleProactivity нь одоогийн байдалд зарлаж болох дээд шат.
	EligibleProactivity string `json:"eligible_proactivity"`
}

func FromRegistryOnceOnlyReport(r registryuc.OnceOnlyReport) RegistryOnceOnlyReportResponse {
	violations := make([]RegistryEvidenceLinkResponse, 0, len(r.Violations))
	for _, e := range r.Violations {
		violations = append(violations, RegistryEvidenceLinkResponse{
			EvidenceID: e.EvidenceID, Code: e.Code, Name: e.Name,
			Required: e.Required, FromCitizen: e.FromCitizen, InKHUR: e.InKHUR,
			OnceOnlyViolation: true, Note: e.Note,
		})
	}
	return RegistryOnceOnlyReportResponse{
		ServiceID: r.ServiceID, ServiceCode: r.ServiceCode, ServiceName: r.ServiceName,
		CitizenDocuments: r.CitizenDocuments, Violations: violations,
		Compliant: r.Compliant, EligibleProactivity: r.EligibleProactivity,
	}
}

// ── Нэгтгэл ─────────────────────────────────────────────────────────────────

type RegistryOverviewResponse struct {
	TotalServices      int            `json:"total_services"`
	PublishedServices  int            `json:"published_services"`
	DraftServices      int            `json:"draft_services"`
	LifeEvents         int            `json:"life_events"`
	Evidences          int            `json:"evidences"`
	EvidencesInKHUR    int            `json:"evidences_in_khur"`
	OnceOnlyViolations int            `json:"once_only_violations"`
	OnceOnlyAnnualHits int            `json:"once_only_annual_hits"`
	ByProactivity      map[string]int `json:"by_proactivity"`
	AvgMaxDays         float64        `json:"avg_max_days"`
}

func FromRegistryOverview(o domain.RegistryOverview) RegistryOverviewResponse {
	if o.ByProactivity == nil {
		o.ByProactivity = map[string]int{}
	}
	return RegistryOverviewResponse{
		TotalServices: o.TotalServices, PublishedServices: o.PublishedServices,
		DraftServices: o.DraftServices, LifeEvents: o.LifeEvents, Evidences: o.Evidences,
		EvidencesInKHUR: o.EvidencesInKHUR, OnceOnlyViolations: o.OnceOnlyViolations,
		OnceOnlyAnnualHits: o.OnceOnlyAnnualHits, ByProactivity: o.ByProactivity,
		AvgMaxDays: o.AvgMaxDays,
	}
}
