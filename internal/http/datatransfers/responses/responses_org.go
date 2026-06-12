// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
)

// OrgResponse нь нэг байгууллагыг клиентэд буцаана.
type OrgResponse struct {
	ID        string     `json:"id"`
	RegNo     string     `json:"reg_no"`
	Name      string     `json:"name"`
	NameLatin string     `json:"name_latin"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// FromOrg нь байгууллагын entity-г хариуны DTO руу буулгана.
func FromOrg(o domain.Organization) OrgResponse {
	return OrgResponse{
		ID:        o.ID,
		RegNo:     o.RegNo,
		Name:      o.Name,
		NameLatin: o.NameLatin,
		CreatedBy: o.CreatedBy,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

// ToOrgList нь байгууллагын жагсаалтыг DTO жагсаалт болгоно.
func ToOrgList(list []domain.Organization) []OrgResponse {
	out := make([]OrgResponse, 0, len(list))
	for _, o := range list {
		out = append(out, FromOrg(o))
	}
	return out
}

// OrgMemberResponse нь нэг гишүүнчлэлийг буцаана.
type OrgMemberResponse struct {
	OrgID     string    `json:"org_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// FromOrgMember нь гишүүнчлэлийн entity-г хариуны DTO руу буулгана.
func FromOrgMember(m domain.OrganizationMembership) OrgMemberResponse {
	return OrgMemberResponse{
		OrgID:     m.OrgID,
		UserID:    m.UserID,
		Role:      m.Role,
		CreatedAt: m.CreatedAt,
	}
}

// ToOrgMemberList нь гишүүдийн жагсаалтыг DTO жагсаалт болгоно.
func ToOrgMemberList(list []domain.OrganizationMembership) []OrgMemberResponse {
	out := make([]OrgMemberResponse, 0, len(list))
	for _, m := range list {
		out = append(out, FromOrgMember(m))
	}
	return out
}
