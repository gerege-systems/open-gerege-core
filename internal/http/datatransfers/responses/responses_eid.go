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
