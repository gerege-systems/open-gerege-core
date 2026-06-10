// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package records

import (
	"template/internal/business/domain"
)

func (u *Users) ToV1Domain() domain.User {
	return domain.User{
		ID:                u.Id,
		Username:          u.Username,
		FirstName:         u.FirstName,
		LastName:          u.LastName,
		FirstNameEn:       u.FirstNameEn,
		LastNameEn:        u.LastNameEn,
		Email:             u.Email,
		Password:          u.Password,
		Active:            u.Active,
		RoleID:            u.RoleId,
		CreatedAt:         u.CreatedAt,
		UpdatedAt:         u.UpdatedAt,
		DeletedAt:         u.DeletedAt,
		PasswordChangedAt: u.PasswordChangedAt,
	}
}

func FromUsersV1Domain(u *domain.User) Users {
	return Users{
		Id:                u.ID,
		Username:          u.Username,
		FirstName:         u.FirstName,
		LastName:          u.LastName,
		FirstNameEn:       u.FirstNameEn,
		LastNameEn:        u.LastNameEn,
		Email:             u.Email,
		Password:          u.Password,
		Active:            u.Active,
		RoleId:            u.RoleID,
		CreatedAt:         u.CreatedAt,
		UpdatedAt:         u.UpdatedAt,
		DeletedAt:         u.DeletedAt,
		PasswordChangedAt: u.PasswordChangedAt,
	}
}

func ToArrayOfUsersV1Domain(u *[]Users) []domain.User {
	var result []domain.User
	for _, val := range *u {
		result = append(result, val.ToV1Domain())
	}
	return result
}
