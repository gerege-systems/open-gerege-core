// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// SuperadminCreateAdminRequest нь POST /superadmin/admins-ийн body — шинэ admin
// бүртгэл үүсгэнэ. Нэр (мн/en) сонголттой; username/email/password заавал.
type SuperadminCreateAdminRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=8,max=128"`
	FirstName   string `json:"first_name" validate:"omitempty,max=100"`
	LastName    string `json:"last_name" validate:"omitempty,max=100"`
	FirstNameEn string `json:"first_name_en" validate:"omitempty,max=100"`
	LastNameEn  string `json:"last_name_en" validate:"omitempty,max=100"`
}
