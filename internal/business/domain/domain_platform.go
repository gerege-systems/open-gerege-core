// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

// Платформын хандалтын горим (platform_settings.access_mode).
const (
	// AccessModePublic — хэн ч Government SSO (eID)-ээр нэвтэрч болно; шинэ иргэн
	// автоматаар бүртгэгдэнэ (одоогийн үндсэн зан төлөв).
	AccessModePublic = "public"
	// AccessModePrivate — зөвхөн админаас урьдчилан бүртгэсэн (national_id/civil_id-
	// ээр тохирох) хэрэглэгч л нэвтэрнэ. Бусад иргэн eID-ээр баталгаажсан ч
	// нэвтрэх эрхгүй (403).
	AccessModePrivate = "private"
)
