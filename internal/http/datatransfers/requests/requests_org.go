// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// CreateOrgRequest нь POST /v1/org-ийн body. reg_no нь улсын бүртгэлийн дугаар;
// name нь монгол нэр; name_latin нь сонголттой латин (галиглсан) нэр.
type CreateOrgRequest struct {
	RegNo     string `json:"reg_no" validate:"required,max=40"`
	Name      string `json:"name" validate:"required,min=2,max=200"`
	NameLatin string `json:"name_latin" validate:"omitempty,max=200"`
}

// EIDOrgRegisterRequest нь POST /v1/users/me/eid/organizations-ийн body — улсын
// бүртгэлээс (XYP) байгууллагыг regNo-гоор хайж, нэвтэрсэн иргэнд (эрх бүхий бол)
// eidmongolia-д төлөөлөл болгон холбоно.
type EIDOrgRegisterRequest struct {
	RegNo string `json:"reg_no" validate:"required,min=4,max=16"`
}

// AddEIDSignerRequest нь POST /v1/users/me/eid/organizations/{regNo}/signers-ийн body —
// байгууллагад өөр иргэнийг (РД) гарын үсэг зурах эрхтэй (MANAGER) төлөөлөгч болгон нэмнэ.
// Нэмэгдэх эрх нь үргэлж MANAGER (eidmongolia талд шийдэгдэнэ).
type AddEIDSignerRequest struct {
	SignerRegNo string `json:"signer_reg_no" validate:"required,min=8,max=20"`
	Role        string `json:"role" validate:"omitempty,max=100"`
}

// AddMemberRequest нь POST /v1/org/{id}/members-ийн body. role хоосон бол
// 'member' болж өгөгдмөлддөг (usecase шийднэ).
type AddMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"omitempty,oneof=owner admin member"`
}

// UpdateMemberRoleRequest нь PUT /v1/org/{id}/members/{userID}-ийн body.
type UpdateMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner admin member"`
}
