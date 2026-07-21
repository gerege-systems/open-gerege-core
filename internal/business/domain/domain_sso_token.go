// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"errors"
	"time"
)

// ErrSSOTokenNotFound нь тухайн хэрэглэгчийн хадгалагдсан SSO токен байхгүй
// (нэвтрэлт offline_access-ээс өмнө болсон, эсвэл устсан) үед буцна. Дуудагч
// үүнийг "дахин нэвтрэх шаардлагатай" төлөв болгон харуулна.
var ErrSSOTokenNotFound = errors.New("sso token not found")

// SSOToken нь иргэний dgov-SSO OAuth токенууд (нээлттэй хэлбэрээр). Repository нь
// эдгээрийг DB-д AES-GCM-ээр шифрлэж хадгална; энэ бүтэц зөвхөн санах ойд байна.
type SSOToken struct {
	AccessToken     string
	RefreshToken    string
	AccessExpiresAt time.Time
}
