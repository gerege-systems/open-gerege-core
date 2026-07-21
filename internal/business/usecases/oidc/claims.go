// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"strings"

	"template/internal/business/domain"
)

// ClaimsForScopes нь олгосон scope-оос хамааран иргэний claims-ыг гаргана.
//
// ЭНЭ БОЛ ГАДААД ГЭРЭЭ: RP-үүд эдгээр нэрсээр (name, national_id, …) хэрэглэгчээ
// таньдаг тул нэр өөрчлөх нь эвдрэл үүсгэнэ. Өмнө нь provider багцад байсныг
// энд зөөв — одоо id_token болон userinfo хоёулаа үүнийг ашиглана.
//
// `sub`-ыг ЭНД тавихгүй — token endpoint нь challenge-ийн subject-ээс тавина.
//
// Claims нь token гаргах МӨЧИД угсрагдана (consent өгсөн мөчид биш) — иргэн
// профайлаа шинэчилбэл дараагийн token шинэ утгыг агуулна.
func ClaimsForScopes(scopes []string, u domain.User) map[string]any {
	claims := map[string]any{}
	for _, s := range scopes {
		switch s {
		case "profile":
			setIfNonEmpty(claims, "name", u.FullName())
			setIfNonEmpty(claims, "given_name", u.FirstName)
			setIfNonEmpty(claims, "family_name", u.LastName)
			setIfNonEmpty(claims, "given_name_en", u.FirstNameEn)
			setIfNonEmpty(claims, "family_name_en", u.LastNameEn)
		case "email":
			setIfNonEmpty(claims, "email", u.Email)
			if u.Email != "" {
				claims["email_verified"] = true
			}
		case "nationalid":
			setIfNonEmpty(claims, "national_id", u.NationalID)
			setIfNonEmpty(claims, "register_number", u.CivilID)
		case "google":
			// Google холболт — ЗӨВХӨН RP "google" scope-ыг хүсэж, иргэн зөвшөөрсөн
			// үед дамжуулна. Scope-гүйгээр болзолгүй дамжуулбал openid-only RP хүртэл
			// иргэний Google и-мэйл/нэр/зургийг зөвшөөрөлгүйгээр авах data-minimization
			// зөрчил үүснэ.
			if strings.TrimSpace(u.GoogleSub) != "" {
				claims["google_sub"] = u.GoogleSub
				setIfNonEmpty(claims, "google_email", u.GoogleEmail)
				setIfNonEmpty(claims, "google_name", u.GoogleName)
				setIfNonEmpty(claims, "google_picture", u.GooglePicture)
			}
		}
	}
	return claims
}

func setIfNonEmpty(m map[string]any, k, v string) {
	if strings.TrimSpace(v) != "" {
		m[k] = v
	}
}
