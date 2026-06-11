// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
)

type UserResponse struct {
	Id           string     `json:"id"`
	Username     string     `json:"username"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	FullName     string     `json:"full_name"`
	FirstNameEn  string     `json:"first_name_en"`
	LastNameEn   string     `json:"last_name_en"`
	FullNameEn   string     `json:"full_name_en"`
	Email        string     `json:"email"`
	RoleId       int        `json:"role_id"`
	Token        string     `json:"token,omitempty"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

func (u *UserResponse) ToV1Domain() domain.User {
	return domain.User{
		ID:        u.Id,
		Username:  u.Username,
		Email:     u.Email,
		RoleID:    u.RoleId,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// FromV1Domain нь хэрэглэгчийн entity-г хариуны DTO руу буулгана. Токен
// талбарууд тэг хэвээр үлдэнэ — entity нь auth артефакт агуулдаггүй.
// /login болон /refresh замуудад FromLoginResult-г ашигла.
func FromV1Domain(u domain.User) UserResponse {
	return UserResponse{
		Id:          u.ID,
		Username:    u.Username,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		FullName:    u.FullName(),
		FirstNameEn: u.FirstNameEn,
		LastNameEn:  u.LastNameEn,
		FullNameEn:  u.FullNameEn(),
		Email:       u.Email,
		RoleId:      u.RoleID,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// FromLoginResponse нь /login + /refresh хариуны хэлбэр юм: хэрэглэгчийн
// талбарууд нь FromV1Domain-тэй ижил бөгөөд дээр нь auth урсгалаас
// шинээр үүсгэсэн токен хос нэмэгдсэн.
func FromLoginResponse(r auth.LoginResponse) UserResponse {
	resp := FromV1Domain(r.User)
	resp.Token = r.AccessToken
	resp.RefreshToken = r.RefreshToken
	return resp
}

func ToResponseList(domains []domain.User) []UserResponse {
	var result []UserResponse

	for _, val := range domains {
		result = append(result, FromV1Domain(val))
	}

	return result
}
