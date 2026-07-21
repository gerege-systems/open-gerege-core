// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Webhook гарын үсгийн header-ууд — peer platform-ууд хоорондоо HMAC-SHA256-оор
// хүсэлтийг баталгаажуулна (JWT-гүй, m2m).
const (
	RelayWebhookSourceHeader = "X-Relay-Source"    // илгээгч platform-ын code
	RelayWebhookSigHeader    = "X-Relay-Signature" // sha256=<hex>
	RelayWebhookEventHeader  = "X-Relay-Event"     // envelope-ийн event төрөл
	RelayWebhookSigPrefix    = "sha256="
)

// RelayWebhookEnvelope нь peer платформ хооронд дамжуулах webhook-ийн бие.
// Дээшээ/доошоо хүсэлт болон хариу дамжуулахад хоёуланд нь ашиглана.
type RelayWebhookEnvelope struct {
	Event       string     `json:"event"`                  // received|forward|fulfilled|breach|...
	SourceCode  string     `json:"source_code"`            // илгээгч платформын code
	ServiceCode string     `json:"service_code,omitempty"` // routing-ийн service_code
	ExternalRef string     `json:"external_ref,omitempty"`
	Title       string     `json:"title,omitempty"`
	Priority    string     `json:"priority,omitempty"`
	Payload     []byte     `json:"payload,omitempty"`
	Result      []byte     `json:"result,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	SentAt      time.Time  `json:"sent_at"`
}

// RelaySignWebhook нь body-г нууц түлхүүрээр HMAC-SHA256 гарын үсэг зурж
// "sha256=<hex>" хэлбэрээр буцаана.
func RelaySignWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return RelayWebhookSigPrefix + hex.EncodeToString(mac.Sum(nil))
}

// RelayVerifyWebhook нь ирсэн гарын үсгийг тогтмол хугацаанд (constant-time)
// шалгана. Хоосон нууц/гарын үсэг бол false.
func RelayVerifyWebhook(secret, signature string, body []byte) bool {
	if secret == "" || signature == "" {
		return false
	}
	expected := RelaySignWebhook(secret, body)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

// RelayNewWebhookSecret нь шинэ platform-д санамсаргүй 64-hex webhook нууц үүсгэнэ.
func RelayNewWebhookSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// RelayIsDemoEndpoint нь endpoint нь бодит HTTP биш (хоосон эсвэл demo://) эсэхийг
// шалгана — тийм бол webhook-ийг гадагш илгээхгүй (demo simulator дотооддоо ажиллана).
func RelayIsDemoEndpoint(endpoint string) bool {
	e := strings.TrimSpace(endpoint)
	return e == "" || strings.HasPrefix(e, "demo://")
}
