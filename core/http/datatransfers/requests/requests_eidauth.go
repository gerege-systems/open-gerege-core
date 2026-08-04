// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// EIDAuthStartRequest нь POST /eid-auth/start-ийн body — QR/device-link-ээр
// эхлүүлнэ. CallbackUrl хоосон бол CROSS-DEVICE (desktop QR).
type EIDAuthStartRequest struct {
	CallbackUrl string `json:"callbackUrl,omitempty" validate:"omitempty,url,max=512"`
}

// EIDAuthStartByNationalIDRequest нь POST /eid-auth/start-id-ийн body —
// иргэний РД-аар бүртгэлтэй төхөөрөмж рүү push илгээнэ.
type EIDAuthStartByNationalIDRequest struct {
	NationalID  string `json:"national_id" validate:"required,max=32"`
	CallbackUrl string `json:"callbackUrl,omitempty" validate:"omitempty,url,max=512"`
}

// EIDAuthPollRequest нь POST /eid-auth/poll-ийн body.
type EIDAuthPollRequest struct {
	SessionID string `json:"session_id" validate:"required,max=128"`
}
