// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// UserIntegration нь хэрэглэгчийн гуравдагч этгээдийн үйлчилгээтэй (Google
// Drive/Meet, Dropbox) холбосон OAuth токеныг төлөөлнө. AccessToken/RefreshToken
// нь storage давхаргад шифрлэгдсэн байдаг — domain нь зөвхөн утгыг зөөдөг,
// шифрлэлтийг usecase давхарга хариуцна.
type UserIntegration struct {
	ID           string
	UserID       string
	Provider     string // "google-drive", "dropbox", "google-meet"
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}
