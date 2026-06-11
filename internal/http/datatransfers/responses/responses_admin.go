// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	"template/internal/business/domain"
)

// AdminUserResponse нь admin хэрэглэгч-удирдлагын жагсаалтын мөр. UserResponse-
// оос ялгаатай нь active статусыг агуулна (token талбаргүй).
type AdminUserResponse struct {
	Id         string     `json:"id"`
	Username   string     `json:"username"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	FullName   string     `json:"full_name"`
	FullNameEn string     `json:"full_name_en"`
	Email      string     `json:"email"`
	RoleId     int        `json:"role_id"`
	Active     bool       `json:"active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

func FromAdminUser(u domain.User) AdminUserResponse {
	return AdminUserResponse{
		Id: u.ID, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName,
		FullName: u.FullName(), FullNameEn: u.FullNameEn(),
		Email: u.Email, RoleId: u.RoleID, Active: u.Active, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func ToAdminUserList(users []domain.User) []AdminUserResponse {
	out := make([]AdminUserResponse, 0, len(users))
	for i := range users {
		out = append(out, FromAdminUser(users[i]))
	}
	return out
}
