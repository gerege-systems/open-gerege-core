// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package records

import (
	"template/internal/business/domain"
)

// derefStr нь nullable (*string) баганыг domain-ийн string руу буулгана —
// NULL нь хоосон string болно.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ptrOrNil нь хоосон string-ийг NULL (nil pointer), бусдыг pointer болгоно —
// eID хэрэглэгчийн хоосон email/password-ийг DB-д NULL болгож хадгалахад.
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (u *Users) ToV1Domain() domain.User {
	return domain.User{
		ID:                  u.Id,
		Username:            u.Username,
		FirstName:           u.FirstName,
		LastName:            u.LastName,
		FirstNameEn:         u.FirstNameEn,
		LastNameEn:          u.LastNameEn,
		Email:               derefStr(u.Email),
		Password:            derefStr(u.Password),
		Active:              u.Active,
		RoleID:              u.RoleId,
		NationalID:          derefStr(u.NationalID),
		CivilID:             derefStr(u.CivilID),
		KYCLevel:            derefStr(u.KYCLevel),
		DocumentNumber:      derefStr(u.DocumentNumber),
		CertSerial:          derefStr(u.CertSerial),
		CertNotBefore:       u.CertNotBefore,
		CertNotAfter:        u.CertNotAfter,
		CertIssuer:          derefStr(u.CertIssuer),
		CertKeyType:         derefStr(u.CertKeyType),
		GoogleSub:           derefStr(u.GoogleSub),
		GoogleEmail:         derefStr(u.GoogleEmail),
		GoogleEmailVerified: u.GoogleEmailVerified,
		GoogleName:          derefStr(u.GoogleName),
		GooglePicture:       derefStr(u.GooglePicture),
		GoogleLinkedAt:      u.GoogleLinkedAt,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
		DeletedAt:           u.DeletedAt,
		PasswordChangedAt:   u.PasswordChangedAt,
	}
}

func FromUsersV1Domain(u *domain.User) Users {
	return Users{
		Id:                  u.ID,
		Username:            u.Username,
		FirstName:           u.FirstName,
		LastName:            u.LastName,
		FirstNameEn:         u.FirstNameEn,
		LastNameEn:          u.LastNameEn,
		Email:               ptrOrNil(u.Email),
		Password:            ptrOrNil(u.Password),
		Active:              u.Active,
		RoleId:              u.RoleID,
		NationalID:          ptrOrNil(u.NationalID),
		CivilID:             ptrOrNil(u.CivilID),
		KYCLevel:            ptrOrNil(u.KYCLevel),
		DocumentNumber:      ptrOrNil(u.DocumentNumber),
		CertSerial:          ptrOrNil(u.CertSerial),
		CertNotBefore:       u.CertNotBefore,
		CertNotAfter:        u.CertNotAfter,
		CertIssuer:          ptrOrNil(u.CertIssuer),
		CertKeyType:         ptrOrNil(u.CertKeyType),
		GoogleSub:           ptrOrNil(u.GoogleSub),
		GoogleEmail:         ptrOrNil(u.GoogleEmail),
		GoogleEmailVerified: u.GoogleEmailVerified,
		GoogleName:          ptrOrNil(u.GoogleName),
		GooglePicture:       ptrOrNil(u.GooglePicture),
		GoogleLinkedAt:      u.GoogleLinkedAt,
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
		DeletedAt:           u.DeletedAt,
		PasswordChangedAt:   u.PasswordChangedAt,
	}
}

func ToArrayOfUsersV1Domain(u *[]Users) []domain.User {
	var result []domain.User
	for _, val := range *u {
		result = append(result, val.ToV1Domain())
	}
	return result
}
