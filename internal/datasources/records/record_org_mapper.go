// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package records

import (
	"template/internal/business/domain"
)

// ToV1Domain нь Organizations record-ийг domain.Organization руу буулгана.
func (o *Organizations) ToV1Domain() domain.Organization {
	return domain.Organization{
		ID:        o.Id,
		RegNo:     o.RegNo,
		Name:      o.Name,
		NameLatin: o.NameLatin,
		CreatedBy: o.CreatedBy,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

// ToArrayOfOrganizationsV1Domain нь record slice-ийг domain slice болгоно.
func ToArrayOfOrganizationsV1Domain(o *[]Organizations) []domain.Organization {
	result := make([]domain.Organization, 0, len(*o))
	for i := range *o {
		result = append(result, (*o)[i].ToV1Domain())
	}
	return result
}

// ToV1Domain нь OrganizationMemberships record-ийг domain руу буулгана.
func (m *OrganizationMemberships) ToV1Domain() domain.OrganizationMembership {
	return domain.OrganizationMembership{
		OrgID:     m.OrgID,
		UserID:    m.UserID,
		Role:      m.Role,
		CreatedAt: m.CreatedAt,
	}
}

// ToArrayOfOrgMembershipsV1Domain нь record slice-ийг domain slice болгоно.
func ToArrayOfOrgMembershipsV1Domain(m *[]OrganizationMemberships) []domain.OrganizationMembership {
	result := make([]domain.OrganizationMembership, 0, len(*m))
	for i := range *m {
		result = append(result, (*m)[i].ToV1Domain())
	}
	return result
}
