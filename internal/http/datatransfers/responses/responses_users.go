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
	// EID нь eID-ээр нэвтэрсэн хэрэглэгчийн identity + сертификатын мэдээлэл.
	// Нууц үгээр бүртгүүлсэн хэрэглэгчид nil (omitempty).
	EID *EIDInfo `json:"eid,omitempty"`
	// Google нь холбогдсон Google account-аас хадгалсан профайл. Google
	// холбоогүй хэрэглэгчид nil (omitempty).
	Google *GoogleInfo `json:"google,omitempty"`
}

// GoogleInfo нь холбогдсон Google account-аас хадгалсан профайл (Dashboard-д харуулна).
type GoogleInfo struct {
	Email         string     `json:"email,omitempty"`
	EmailVerified bool       `json:"email_verified"`
	Name          string     `json:"name,omitempty"`
	Picture       string     `json:"picture,omitempty"`
	LinkedAt      *time.Time `json:"linked_at,omitempty"`
}

// EIDInfo нь eidmongolia.mn-ээс login үед авсан бүх нээлттэй мэдээлэл.
type EIDInfo struct {
	CivilID        string          `json:"civil_id,omitempty"`
	NationalID     string          `json:"national_id,omitempty"` // регистрийн дугаар
	KYCLevel       string          `json:"kyc_level,omitempty"`   // сертификатын түвшин
	DocumentNumber string          `json:"document_number,omitempty"`
	Certificate    *EIDCertificate `json:"certificate,omitempty"`
}

// EIDCertificate нь login COMPLETE-ийн cert.value (DER)-ээс задалсан хэсэг.
type EIDCertificate struct {
	Serial    string     `json:"serial,omitempty"`
	NotBefore *time.Time `json:"not_before,omitempty"`
	NotAfter  *time.Time `json:"not_after,omitempty"`
	Issuer    string     `json:"issuer,omitempty"`
	KeyType   string     `json:"key_type,omitempty"`
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
		EID:         eidInfoOf(u),
		Google:      googleInfoOf(u),
	}
}

// googleInfoOf нь Google холбогдсон (google_sub байгаа) бол GoogleInfo блок
// үүсгэнэ; эс бөгөөс nil (хариунд google талбар огт орохгүй).
func googleInfoOf(u domain.User) *GoogleInfo {
	if u.GoogleSub == "" {
		return nil
	}
	return &GoogleInfo{
		Email:         u.GoogleEmail,
		EmailVerified: u.GoogleEmailVerified,
		Name:          u.GoogleName,
		Picture:       u.GooglePicture,
		LinkedAt:      u.GoogleLinkedAt,
	}
}

// eidInfoOf нь eID identity талбар байвал EIDInfo блок үүсгэнэ; эс бөгөөс nil
// (нууц үгээр бүртгүүлсэн хэрэглэгчийн хариунд eid талбар огт орохгүй).
func eidInfoOf(u domain.User) *EIDInfo {
	if u.CivilID == "" && u.NationalID == "" && u.DocumentNumber == "" {
		return nil
	}
	info := &EIDInfo{
		CivilID:        u.CivilID,
		NationalID:     u.NationalID,
		KYCLevel:       u.KYCLevel,
		DocumentNumber: u.DocumentNumber,
	}
	if u.CertSerial != "" || u.CertNotAfter != nil || u.CertIssuer != "" {
		info.Certificate = &EIDCertificate{
			Serial:    u.CertSerial,
			NotBefore: u.CertNotBefore,
			NotAfter:  u.CertNotAfter,
			Issuer:    u.CertIssuer,
			KeyType:   u.CertKeyType,
		}
	}
	return info
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

// EIDStartResponse нь POST /auth/eid/start-ийн хариу — клиент QR/deep-link
// харуулж, /eid/poll руу session_id-г дамжуулна.
type EIDStartResponse struct {
	SessionID        string `json:"session_id"`
	DeviceLinkURL    string `json:"device_link_url"`
	VerificationCode string `json:"verification_code"`
	ExpiresAt        string `json:"expires_at"`
}

// FromEIDStartResponse нь usecase-ийн EIDStartResponse-ийг DTO рүү буулгана.
func FromEIDStartResponse(r auth.EIDStartResponse) EIDStartResponse {
	return EIDStartResponse{
		SessionID:        r.SessionID,
		DeviceLinkURL:    r.DeviceLinkURL,
		VerificationCode: r.VerificationCode,
		ExpiresAt:        r.ExpiresAt,
	}
}

// EIDPollResponse нь POST /auth/eid/poll-ийн хариу. state нь IdP-ийн session
// төлөв (RUNNING / COMPLETE / EXPIRED / REFUSED). COMPLETE үед UserResponse-ийн
// бүх талбар (token / refresh_token-ийг оруулаад) /login-той ИЖИЛ хэлбэрээр
// бөглөгдөнө — frontend BFF-ийн data.token / data.refresh_token уншилт өөрчлөгдөхгүй.
type EIDPollResponse struct {
	State string `json:"state"`
	UserResponse
}

// FromEIDPollResponse нь usecase-ийн EIDPollResponse-ийг DTO рүү буулгана.
// COMPLETE биш үед зөвхөн state бөглөнө (хэрэглэгчийн талбарууд хоосон).
func FromEIDPollResponse(r auth.EIDPollResponse) EIDPollResponse {
	out := EIDPollResponse{State: r.State}
	if r.State == "COMPLETE" {
		out.UserResponse = FromV1Domain(r.User)
		out.UserResponse.Token = r.AccessToken
		out.UserResponse.RefreshToken = r.RefreshToken
	}
	return out
}

func ToResponseList(domains []domain.User) []UserResponse {
	var result []UserResponse

	for _, val := range domains {
		result = append(result, FromV1Domain(val))
	}

	return result
}

// GoogleLoginResponse нь POST /auth/google-ийн хариу. Linked=true бол User
// (токентой) дүүрэн; false бол link_token + email (eID-ээр баталгаажуулах).
type GoogleLoginResponse struct {
	Linked    bool          `json:"linked"`
	User      *UserResponse `json:"user,omitempty"`
	LinkToken string        `json:"link_token,omitempty"`
	Email     string        `json:"email,omitempty"`
}

// FromGoogleLoginResponse нь usecase-ийн үр дүнг DTO рүү буулгана.
func FromGoogleLoginResponse(r auth.GoogleLoginResponse) GoogleLoginResponse {
	if r.Linked {
		u := FromLoginResponse(r.Login)
		return GoogleLoginResponse{Linked: true, User: &u}
	}
	return GoogleLoginResponse{Linked: false, LinkToken: r.LinkToken, Email: r.Email}
}
