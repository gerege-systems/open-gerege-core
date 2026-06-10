// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"template/internal/config"
)

var (
	trustedNetsOnce sync.Once
	trustedNets     []*net.IPNet
)

// trustedProxyNets нь TRUSTED_PROXIES config-г нэг удаа задлан *net.IPNet
// жагсаалт болгож кэшэлнэ. Дан IP-г /32 (IPv4) эсвэл /128 (IPv6) болгоно.
func trustedProxyNets() []*net.IPNet {
	trustedNetsOnce.Do(func() {
		for _, entry := range config.AppConfig.TrustedProxiesList() {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if !strings.Contains(entry, "/") {
				if ip := net.ParseIP(entry); ip != nil {
					if ip.To4() != nil {
						entry += "/32"
					} else {
						entry += "/128"
					}
				}
			}
			if _, n, err := net.ParseCIDR(entry); err == nil {
				trustedNets = append(trustedNets, n)
			}
		}
	})
	return trustedNets
}

func ipInTrusted(ipStr string, nets []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func hostOf(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// clientIP нь хүсэлтийн жинхэнэ клиент IP-г аюулгүйгээр тодорхойлно.
// X-Forwarded-For-д ЗӨВХӨН холболт өөрөө итгэмжит proxy (TRUSTED_PROXIES)
// байх үед итгэнэ — тэр үед XFF-г баруунаас зүүн тийш гүйж, итгэмжит
// hop-уудыг алгасаад анхны итгэмжгүй (= жинхэнэ клиент) хаягийг буцаана.
// Итгэмжит proxy тохируулаагүй эсвэл peer итгэмжгүй бол RemoteAddr-ийн
// host-г шууд буцаана — ингэснээр халдагч XFF тавиад rate-limit/audit-г
// хуурч чадахгүй (access-log болон rate-limit middleware хуваалцана).
func clientIP(r *http.Request) string {
	remote := hostOf(r.RemoteAddr)
	nets := trustedProxyNets()
	if len(nets) == 0 || !ipInTrusted(remote, nets) {
		return remote
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remote
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if ipInTrusted(ip, nets) {
			continue
		}
		return ip
	}
	return remote
}
