// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Config-ийн pure функцүүдийн unit тест (white-box): TrustedProxiesList /
// AllowedOriginsList-ийн задлалт + production-ийн аюулгүй анхдагч, sslModeOf-ийн
// URL болон keyword DSN хоёр хэлбэрээс sslmode гаргалт (production TLS guard-ийн үндэс).
package config

import (
	"testing"

	"template/internal/constants"
)

func TestTrustedProxiesList(t *testing.T) {
	cases := map[string][]string{
		"":                          nil,
		"127.0.0.1":                 {"127.0.0.1"},
		"127.0.0.1, 172.16.0.0/12 ": {"127.0.0.1", "172.16.0.0/12"},
		" 10.0.0.1 ,, 10.0.0.2 ":    {"10.0.0.1", "10.0.0.2"}, // хоосон хэсгүүд алгасагдана
	}
	for in, want := range cases {
		c := &Config{TrustedProxies: in}
		got := c.TrustedProxiesList()
		if !equalSlice(got, want) {
			t.Errorf("TrustedProxiesList(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestAllowedOriginsList(t *testing.T) {
	t.Run("empty in dev → wildcard", func(t *testing.T) {
		c := &Config{AllowedOrigins: "", Environment: "development"}
		if got := c.AllowedOriginsList(); !equalSlice(got, []string{"*"}) {
			t.Errorf("dev default = %v, want [*]", got)
		}
	})
	t.Run("empty in production → nil (no wildcard)", func(t *testing.T) {
		c := &Config{AllowedOrigins: "", Environment: constants.EnvironmentProduction}
		if got := c.AllowedOriginsList(); got != nil {
			t.Errorf("production default = %v, want nil (wildcard хориотой)", got)
		}
	})
	t.Run("explicit list is parsed", func(t *testing.T) {
		c := &Config{AllowedOrigins: "https://a.mn, https://b.mn", Environment: constants.EnvironmentProduction}
		if got := c.AllowedOriginsList(); !equalSlice(got, []string{"https://a.mn", "https://b.mn"}) {
			t.Errorf("= %v", got)
		}
	})
}

func TestSslModeOf(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@h:5432/db?sslmode=verify-full": "verify-full",
		"postgresql://u@h/db?sslmode=disable":          "disable",
		"host=db port=5432 user=u sslmode=require":     "require",
		"host=db user=u dbname=x":                      "", // sslmode байхгүй
		"":                                             "",
	}
	for conn, want := range cases {
		if got := sslModeOf(conn); got != want {
			t.Errorf("sslModeOf(%q) = %q, want %q", conn, got, want)
		}
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
