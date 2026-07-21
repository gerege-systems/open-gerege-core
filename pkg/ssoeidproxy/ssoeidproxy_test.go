// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// SSO eID proxy client unit тест: {data} дугтуй задлалт, snake_case wire →
// eid домэйн буулгалт, Bearer header, статус-кодын алдаа буулгалт.
package ssoeidproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"template/pkg/eid"
)

// newTestClient нь handler-тай httptest server дээр чиглэсэн client буцаана.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

func TestSummaryMapping(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok-123" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/summary" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"given_name":"Бат","surname":"Дорж","certificates":{"valid":2,"revoked":1,"expired":0,"suspended":0,"total":3},"activity":{"authentication":5,"signature":2},"devices_active":1,"devices_total":2,"representation_count":4}}`))
	})
	res, err := c.Summary(context.Background(), "tok-123")
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if res.GivenName != "Бат" || res.Surname != "Дорж" {
		t.Errorf("name = %q %q", res.GivenName, res.Surname)
	}
	if res.Certificates.Total != 3 || res.Certificates.Valid != 2 {
		t.Errorf("cert counts = %+v", res.Certificates)
	}
	if res.Activity.Authentication != 5 || res.DevicesActive != 1 || res.DevicesTotal != 2 || res.RepresentationCount != 4 {
		t.Errorf("summary = %+v", res)
	}
}

func TestCertificatesMapping(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"counts":{"valid":1,"revoked":0,"expired":0,"suspended":0,"total":1},"certificates":[{"document_number":"D1","type":"AUTH","serial_number":"1a","certificate_level":"QUALIFIED","status":"VALID","not_before":"2026-01-01T00:00:00Z","not_after":"2027-01-01T00:00:00Z","issuer_dn":"CN=CA"}]}}`))
	})
	res, err := c.Certificates(context.Background(), "tok")
	if err != nil {
		t.Fatalf("Certificates: %v", err)
	}
	if len(res.Certificates) != 1 {
		t.Fatalf("certs len = %d", len(res.Certificates))
	}
	cert := res.Certificates[0]
	if cert.DocumentNumber != "D1" || cert.Type != "AUTH" || cert.Status != "VALID" || cert.IssuerDn != "CN=CA" {
		t.Errorf("cert = %+v", cert)
	}
	if cert.NotBefore.IsZero() || cert.NotAfter.IsZero() {
		t.Errorf("cert times not parsed: %+v", cert)
	}
}

func TestActivityQueryParams(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("offset") != "20" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":{"counts":{"authentication":3,"signature":1},"sessions":[{"session_id":"s1","flow":"AUTHENTICATION","outcome":"OK","timestamp":"2026-02-01T00:00:00Z"}],"total":4}}`))
	})
	res, err := c.Activity(context.Background(), "tok", 10, 20)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if res.Total != 4 || res.Counts.Authentication != 3 || len(res.Sessions) != 1 {
		t.Errorf("activity = %+v", res)
	}
	if res.Sessions[0].SessionID != "s1" || res.Sessions[0].Timestamp.IsZero() {
		t.Errorf("session = %+v", res.Sessions[0])
	}
}

func TestDevicesMapping(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"devices":[{"document_number":"D1","platform":"APNS","enrolled_at":"2026-01-01T00:00:00Z","active":true}],"active_count":1,"total":1}}`))
	})
	res, err := c.Devices(context.Background(), "tok")
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if res.ActiveCount != 1 || len(res.Devices) != 1 || res.Devices[0].DocumentNumber != "D1" || !res.Devices[0].Active {
		t.Errorf("devices = %+v", res)
	}
	if res.Devices[0].EnrolledAt.IsZero() {
		t.Errorf("enrolledAt not parsed: %+v", res.Devices[0])
	}
}

func TestStatusErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"unauthorized", http.StatusUnauthorized, ErrTokenExpired},
		{"forbidden", http.StatusForbidden, eid.ErrPKINotPermitted},
		{"disabled", http.StatusServiceUnavailable, ErrProxyDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			})
			_, err := c.Summary(context.Background(), "tok")
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNotFoundIsEmpty(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	res, err := c.Summary(context.Background(), "tok")
	if err != nil {
		t.Fatalf("404 should not error: %v", err)
	}
	if res == nil || res.GivenName != "" {
		t.Errorf("404 should yield zero-value summary, got %+v", res)
	}
}
