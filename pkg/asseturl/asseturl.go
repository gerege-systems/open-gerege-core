// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package asseturl нь тамга/гарын үсгийн зургийн URL-ийг шалгаж (allowlist),
// серверийн зүгээс татах үед SSRF (дотоод сүлжээ/metadata руу хандах) болон
// javascript:/data: зэрэг аюултай схемээс сэргийлнэ. Зургийг BFF нь Google
// Drive-д байршуулж lh3.googleusercontent.com/d/<id> хэлбэрийн URL хадгалдаг тул
// зөвхөн Google-ийн найдвартай host-ыг зөвшөөрнө.
package asseturl

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// hostAllowed нь host-ыг найдвартай Google жагсаалттай тулгана. Урд талд цэгтэй
// suffix шалгалт нь "evil-googleusercontent.com" мэтийг зөвшөөрөхгүй.
func hostAllowed(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(h, ':'); i >= 0 { // порт хас
		h = h[:i]
	}
	switch h {
	case "drive.google.com", "drive.usercontent.google.com", "googleusercontent.com":
		return true
	}
	return strings.HasSuffix(h, ".googleusercontent.com")
}

// Validate нь хадгалах/татах зургийн URL-ийг шалгана: заавал https схем + найдвартай
// host. Буруу бол тодорхой алдаа буцаана (дуудагч apperror болгон боож болно).
func Validate(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("asseturl: invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("asseturl: only https scheme is allowed, got %q", u.Scheme)
	}
	if !hostAllowed(u.Hostname()) {
		return fmt.Errorf("asseturl: host %q is not in the allowlist", u.Hostname())
	}
	return nil
}

// SafeClient нь зөвхөн найдвартай host руу хүсэлт илгээх http.Client буцаана.
// Redirect бүрийн зорилтот host-ыг ДАХИН шалгадаг тул allowlist-д таарсан Google
// host-оос дотоод хаяг руу үсрэх (SSRF) боломжгүй.
func SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("asseturl: too many redirects")
			}
			if req.URL.Scheme != "https" || !hostAllowed(req.URL.Hostname()) {
				return fmt.Errorf("asseturl: redirect to disallowed target %q", req.URL.Redacted())
			}
			return nil
		},
	}
}
