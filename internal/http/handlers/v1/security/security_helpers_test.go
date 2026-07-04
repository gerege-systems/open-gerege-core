// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Security handler-ийн туслах логикийн white-box тест: clientIP нь
// X-Forwarded-For-ийн эхний хаягийг (proxy chain) авах, байхгүй бол RemoteAddr-
// аас port-гүй host гаргах; parseIntDefault-ийн задлалт.
package security

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	t.Run("X-Forwarded-For эхний хаяг", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1, 172.16.0.2")
		if got := clientIP(r); got != "203.0.113.7" {
			t.Errorf("clientIP = %q, want 203.0.113.7", got)
		}
	})
	t.Run("XFF ганц хаяг", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("X-Forwarded-For", " 198.51.100.9 ")
		if got := clientIP(r); got != "198.51.100.9" {
			t.Errorf("clientIP = %q", got)
		}
	})
	t.Run("XFF байхгүй → RemoteAddr host", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "192.0.2.44:54321"
		if got := clientIP(r); got != "192.0.2.44" {
			t.Errorf("clientIP = %q, want 192.0.2.44", got)
		}
	})
}

func TestParseIntDefault(t *testing.T) {
	cases := []struct {
		in  string
		def int
		out int
	}{
		{"", 50, 50},
		{"25", 50, 25},
		{"abc", 50, 50},
		{"0", 50, 0},
		{"-5", 50, -5},
	}
	for _, tc := range cases {
		if got := parseIntDefault(tc.in, tc.def); got != tc.out {
			t.Errorf("parseIntDefault(%q,%d) = %d, want %d", tc.in, tc.def, got, tc.out)
		}
	}
}
