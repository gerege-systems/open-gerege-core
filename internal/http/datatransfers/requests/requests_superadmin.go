// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// SuperadminCreateAdminRequest нь POST /superadmin/admins-ийн body — шинэ admin
// бүртгэл үүсгэнэ. Нэр (мн/en) сонголттой; username/email/password заавал.
// SuperadminAddAdminByRegisterRequest нь регистрийн дугаараар байгаа хэрэглэгчийг
// admin болгох хүсэлт.
type SuperadminAddAdminByRegisterRequest struct {
	Register string `json:"register" validate:"required,min=8,max=20"`
}

type SuperadminCreateAdminRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=8,max=128"`
	FirstName   string `json:"first_name" validate:"omitempty,max=100"`
	LastName    string `json:"last_name" validate:"omitempty,max=100"`
	FirstNameEn string `json:"first_name_en" validate:"omitempty,max=100"`
	LastNameEn  string `json:"last_name_en" validate:"omitempty,max=100"`
}

// SuperadminAccessModeRequest нь PUT /superadmin/access-mode-ийн body — платформын
// хандалтын горим. mode нь 'public' эсвэл 'private'.
type SuperadminAccessModeRequest struct {
	Mode string `json:"mode" validate:"required,oneof=public private"`
}
