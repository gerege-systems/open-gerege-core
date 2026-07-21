// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// GeregeCloud Verify client-ийн unit тест: httptest сервер дээр Send/Check-ийн
// wire хэлбэр, X-API-Key header, request_id задлалт, approved/non-approved/4xx/
// 5xx статусын буулгалт, apiKey-гүй үед fail-fast, өгөгдмөл channel/base-ийг
// шалгана.
package verify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "gck_test_key", "email")
}

func TestSend(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/verify/send" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "gck_test_key" {
			t.Errorf("api key header = %q", got)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["to"] != "user@example.com" || body["channel"] != "sms" {
			t.Errorf("body = %+v", body)
		}
		_, _ = io.WriteString(w, `{"request_id":"gcv_123"}`)
	})

	// channel аргумент нь client-ийн default-ыг дардаг.
	id, err := c.Send(context.Background(), "user@example.com", "sms")
	if err != nil {
		t.Fatal(err)
	}
	if id != "gcv_123" {
		t.Errorf("request_id = %s", id)
	}
}

func TestSendUsesDefaultChannelWhenEmpty(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["channel"] != "email" {
			t.Errorf("default channel = %q, want email", body["channel"])
		}
		_, _ = io.WriteString(w, `{"request_id":"x"}`)
	})
	if _, err := c.Send(context.Background(), "a@b.mn", ""); err != nil {
		t.Fatal(err)
	}
}

func TestSendEmptyRequestIDIsError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"request_id":""}`)
	})
	if _, err := c.Send(context.Background(), "a@b.mn", ""); err == nil {
		t.Fatal("expected error on empty request_id")
	}
}

func TestSendServerErrorSurfaces(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	if _, err := c.Send(context.Background(), "a@b.mn", ""); err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"approved", 200, `{"status":"approved"}`, false},
		{"2xx not approved", 200, `{"status":"pending"}`, true},
		{"4xx wrong code", 400, `{"status":"invalid"}`, true},
		{"5xx surfaces as error", 500, `{}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			err := c.Check(context.Background(), "gcv_123", "123456")
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// 4xx / 2xx-non-approved нь ErrNotApproved sentinel байх ёстой (5xx биш).
			if tc.status < 500 && tc.wantErr && err != ErrNotApproved {
				t.Errorf("want ErrNotApproved, got %v", err)
			}
		})
	}
}

func TestNoAPIKeyFailsFast(t *testing.T) {
	// apiKey хоосон бол HTTP хийхгүйгээр шууд алдаа буцаана.
	c := NewClient("http://127.0.0.1:0", "", "email")
	if _, err := c.Send(context.Background(), "a@b.mn", ""); err == nil {
		t.Error("Send: expected 'not configured' error")
	}
	if err := c.Check(context.Background(), "r", "c"); err == nil {
		t.Error("Check: expected 'not configured' error")
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("", "k", "")
	if c.base != strings.TrimRight(defaultBase, "/") {
		t.Errorf("base default = %s", c.base)
	}
	if c.channel != "email" {
		t.Errorf("channel default = %s", c.channel)
	}
}
