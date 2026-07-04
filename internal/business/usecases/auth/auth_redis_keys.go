// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import "fmt"

// Auth domain-ийн ашигладаг Redis key-ийн угтварууд (prefix). Бичгийн алдаа
// бичигчийг түүний уншигчаас чимээгүйхэн салгахаас сэргийлэх, мөн энэ багцаас
// гадуурх адаптерууд (ялангуяа auth middleware) format string-ийг дахин
// хэрэгжүүлэхийн оронд яг ижил нэрсийг дахин ашиглахын тулд төвлөрүүлсэн.
const (
	prefixRefresh        = "refresh:"
	prefixUserOTP        = "user_otp:"
	prefixOTPAttempts    = "otp_attempts:"
	prefixLoginAttempts  = "login_attempts:"
	prefixForgotAttempts = "forgot_attempts:"
	prefixResetRequest   = "pwd_reset_req:"
	prefixPasswordCutoff = "pwd_cutoff:"
	prefixAccessDeny     = "access_deny:"
)

// RefreshKey нь refresh токены jti бичлэгүүдийг хүрээлдэг; байхгүй ⇒ хүчингүй болсон.
func RefreshKey(jti string) string {
	return fmt.Sprintf("%s%s", prefixRefresh, jti)
}

// UserOTPKey нь идэвхгүй бүртгэлийн амьд 6 оронтой OTP-г хадгална.
func UserOTPKey(email string) string {
	return fmt.Sprintf("%s%s", prefixUserOTP, email)
}

// ResetRequestKey нь нууц үг сэргээх OTP-ийн GeregeCloud Verify request_id-г
// email тус бүрд хадгална (ResetPassword /check-д ашиглана).
func ResetRequestKey(email string) string {
	return fmt.Sprintf("%s%s", prefixResetRequest, email)
}

// OTPAttemptsKey нь email тус бүрийн амжилтгүй VerifyOTP оролдлогуудыг тоолно.
func OTPAttemptsKey(email string) string {
	return fmt.Sprintf("%s%s", prefixOTPAttempts, email)
}

// LoginAttemptsKey нь brute-force түгжих цонхонд зориулж email тус бүрийн
// амжилтгүй Login оролдлогуудыг тоолно.
func LoginAttemptsKey(email string) string {
	return fmt.Sprintf("%s%s", prefixLoginAttempts, email)
}

// ForgotAttemptsKey нь email тус бүрд /password/forgot-ийг rate-limit хийнэ.
func ForgotAttemptsKey(email string) string {
	return fmt.Sprintf("%s%s", prefixForgotAttempts, email)
}

// TokenCutoffKey нь энэ хэрэглэгчид олгогдсон аливаа access токеныг хүчингүй
// гэж тооцох тасалбар цэгийг unix-секундээр хадгална. Auth middleware үүнийг
// баталгаажсан хүсэлт бүр дээр уншдаг; ChangePassword болон ResetPassword нь
// үүнийг бичдэг.
func TokenCutoffKey(userID string) string {
	return fmt.Sprintf("%s%s", prefixPasswordCutoff, userID)
}

// AccessDenyKey нь logout хийсэн access токены jti-г токены үлдсэн амьдрах
// хугацаагаар хадгална; байгаа ⇒ хүчингүй (auth middleware хүсэлт бүрд шалгана).
func AccessDenyKey(jti string) string {
	return fmt.Sprintf("%s%s", prefixAccessDeny, jti)
}

// GoogleLinkKey нь Google-ээр эхний удаа нэвтэрсэн хэрэглэгчийн eID-ээр
// холбогдохыг хүлээж буй богино хугацааны токеныг (→ google_sub) хадгална.
func GoogleLinkKey(token string) string {
	return fmt.Sprintf("google_link:%s", token)
}
