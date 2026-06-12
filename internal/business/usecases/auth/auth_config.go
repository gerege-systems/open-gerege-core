// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import "time"

// Config нь auth use case-д шаардлагатай тохиргооны хэсэг юм. Үүнийг
// NewUsecase-ээр дамжуулан inject хийснээр энэ багц internal/config-оос ямар
// нэг хамааралгүй хэвээр үлддэг — composition root нь env тохиргоог auth
// domain-ийн анхаардаг хэлбэр рүү хувиргадаг.
type Config struct {
	// OTPMaxAttempts нь VerifyOTP-ийн түгжих (lockout) босго юм. OTP-ийн
	// цонхон дотор ийм олон удаа амжилтгүй болсны дараа зөв кодтой байсан ч
	// email түгжигдэнэ.
	OTPMaxAttempts int
	// OTPTTL нь OTP код (болон түүний оролдлогын тоолуур) Redis-д дуусахаасаа
	// өмнө хэр удаан амьд байхыг заана.
	OTPTTL time.Duration
	// PasswordResetTTL нь нууц үг сэргээх OTP-ийн Verify request_id Redis-д хэр
	// удаан амьд байхыг заана. 30 минут нь зохистой анхдагч утга — email очиж
	// ирэх хугацаанд хүрэлцэхүйц урт, алдагдсан код хурдан дуусахуйц богино.
	PasswordResetTTL time.Duration
	// BcryptCost нь нууц үг солих/шинэчлэх үед domain.User.ChangePassword руу
	// дамжуулагдана. Дуудагч (DI) нь app config-оос inject хийдэг.
	BcryptCost int
	// LoginMaxAttempts нь /auth/login-ийн түгжих (lockout) босго юм. (Email тус
	// бүрд, LoginLockoutTTL дотор) ийм олон удаа амжилтгүй болсны дараа email нь
	// зөв нууц үгтэй байсан ч үлдсэн цонхны турш түгжигдэнэ — per-IP rate limit-д
	// үл харагдах тархсан IP-уудаас ирэх удаан brute-force-ийг таслан зогсооно.
	LoginMaxAttempts int
	// LoginLockoutTTL нь түгжих цонх хэр удаан үргэлжлэхийг, мөн email тус
	// бүрийн амжилтгүй оролдлогын тоолуур хэр удаан амьд байхыг заана. 15м нь
	// зохистой анхдагч утга; brute force-ийг таслахад хангалттай урт, бичгийн
	// алдаатай жинхэнэ хэрэглэгч бүрмөсөн хаагдахааргүй богино.
	LoginLockoutTTL time.Duration
	// ForgotMaxAttempts нь нэг email ForgotLockoutTTL дотор хичнээн
	// /password/forgot дуудлага өдөөж болохыг хязгаарлана. GeregeCloud Verify
	// руу OTP илгээлтийг урвуулан ашиглахаас (гадагшаа email/SMS спамаар DOS
	// хийх, төлбөр шатаах) болон халдагчийн өдөөсөн reset-код эргэлтээс хамгаална.
	ForgotMaxAttempts int
	// ForgotLockoutTTL нь /password/forgot-ийн rate-limit-ийн цонх юм.
	ForgotLockoutTTL time.Duration

	// EIDCallbackURL нь eID нэвтрэлт амжилттай болсны дараа IdP буцаах URL.
	// Энэ нь IdP-ийн allowlist-д бүртгэгдсэн байх ёстой (composition root нь
	// config-оос дамжуулна).
	EIDCallbackURL string
	// EIDDisplayText нь IdP/гар утсан дээр харагдах RP-ийн нэр/тайлбар.
	EIDDisplayText string
}
