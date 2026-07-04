// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// eID Mongolia v3 client-ийн unit тест: httptest сервер дээр бодит wire
// хэлбэрийг (anonymous/notification initiate + session long-poll) дуурайж,
// client нь QR device-link угсрах, vc (string/object)-ийг задлах,
// session state → template төлөв рүү (COMPLETE/EXPIRED/REFUSED) буулгах,
// person блокоос identity гаргахыг шалгана.
package eid

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testUUID   = "c4f371c3-20bd-462e-8d97-5bc4a20fde08"
	testName   = "template-web"
	testSecret = "rp_sk_test123"
)

// newTestClient нь өгөгдсөн handler руу чиглэсэн client буцаана.
func newTestClient(t *testing.T, h http.HandlerFunc) (Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, testUUID, testName, testSecret, "ADVANCED"), srv
}

func TestQRInitiate(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authentication/device-link/anonymous" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testSecret {
			t.Errorf("auth header = %q", got)
		}
		var body authInitiateBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.RelyingPartyUUID != testUUID || body.RelyingPartyName != testName {
			t.Errorf("rp identity not sent: %+v", body)
		}
		// Auth-д challenge талбар нь rpChallenge (hash БИШ) — регрессийн хамгаалалт.
		if body.RPChallenge == "" || body.SignatureProtocol != "ACSP_V2" {
			t.Errorf("challenge fields wrong: %+v", body)
		}
		if len(body.Interactions) != 1 || body.Interactions[0].Type != "displayTextAndPIN" || body.Interactions[0].DisplayText60 != "Нэвтрэх" {
			t.Errorf("interactions wrong: %+v", body.Interactions)
		}
		_, _ = io.WriteString(w, `{"sessionID":"sess-1","sessionToken":"tok-abc","sessionSecret":"s","deviceLinkBase":"https://eidmongolia.mn/dl","vc":"7270"}`)
	})

	res, err := c.QRInitiate(context.Background(), "Нэвтрэх", "", "nonce")
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "sess-1" {
		t.Errorf("sessionID = %s", res.SessionID)
	}
	if res.VerificationCode != "7270" {
		t.Errorf("vc = %s", res.VerificationCode)
	}
	// QR агуулга нь ЗҮГЭЭР session UUID (ажилладаг eidmongolia.mn/demo-той ижил).
	if res.DeviceLinkURL != "sess-1" {
		t.Errorf("QR content = %q, want bare sessionID %q", res.DeviceLinkURL, "sess-1")
	}
}

func TestInitiateByNationalID(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/authentication/notification/etsi/PNOMN-") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.HasSuffix(r.URL.Path, "УБ12345678") {
			t.Errorf("national id not in path: %s", r.URL.Path)
		}
		// notification нь vc-г object хэлбэрээр буцаана.
		_, _ = io.WriteString(w, `{"sessionID":"sess-2","vc":{"type":"alphaNumeric4","value":"0489"}}`)
	})

	res, err := c.Initiate(context.Background(), "УБ12345678", "Нэвтрэх", "nonce")
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "sess-2" {
		t.Errorf("sessionID = %s", res.SessionID)
	}
	if res.VerificationCode != "0489" {
		t.Errorf("vc (object form) = %s", res.VerificationCode)
	}
	if res.DeviceLinkURL != "" {
		t.Errorf("push flow should have no device link, got %s", res.DeviceLinkURL)
	}
}

func TestInitiateRejected(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"citizen not found"}`)
	})
	_, err := c.Initiate(context.Background(), "УБ00000000", "", "")
	if !errors.Is(err, ErrInitiateRejected) {
		t.Fatalf("want ErrInitiateRejected, got %v", err)
	}
}

func TestSessionStateMapping(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantState string
		wantCivil string
	}{
		{"running", `{"state":"RUNNING"}`, StateRunning, ""},
		{
			"complete ok", `{"state":"COMPLETE","result":{"endResult":"OK"},"cert":{"certificateLevel":"QUALIFIED"},"person":{"givenName":"Бат","surname":"Дорж","givenNameEn":"Bat","surnameEn":"Dorj","civilId":"УБ99887766","regNo":"1234567"}}`,
			StateComplete, "УБ99887766",
		},
		{"complete timeout → expired", `{"state":"COMPLETE","result":{"endResult":"TIMEOUT"}}`, StateExpired, ""},
		{"complete user refused → refused", `{"state":"COMPLETE","result":{"endResult":"USER_REFUSED"}}`, StateRefused, ""},
		{"complete wrong vc → refused", `{"state":"COMPLETE","result":{"endResult":"WRONG_VC"}}`, StateRefused, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			})
			res, err := c.Session(context.Background(), "sess-1", 1000)
			if err != nil {
				t.Fatal(err)
			}
			if res.State != tc.wantState {
				t.Errorf("state = %s, want %s", res.State, tc.wantState)
			}
			if tc.wantCivil != "" {
				if res.Identity == nil || res.Identity.CivilID != tc.wantCivil {
					t.Errorf("identity civilID = %+v, want %s", res.Identity, tc.wantCivil)
				}
				if res.Identity.GivenName != "Бат" || res.Identity.SurnameEn != "Dorj" {
					t.Errorf("person names not mapped: %+v", res.Identity)
				}
				if res.Identity.KYCLevel != "QUALIFIED" {
					t.Errorf("kyc level = %s", res.Identity.KYCLevel)
				}
			} else if res.Identity != nil {
				t.Errorf("non-OK state should carry no identity, got %+v", res.Identity)
			}
		})
	}
}

func TestSessionCompleteOKWithoutPersonErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"state":"COMPLETE","result":{"endResult":"OK"}}`)
	})
	_, err := c.Session(context.Background(), "sess-1", 1000)
	if err == nil {
		t.Fatal("expected error when COMPLETE+OK has no person block")
	}
}

func TestParseVC(t *testing.T) {
	cases := map[string]string{
		`"7270"`: "7270",
		`{"type":"alphaNumeric4","value":"0489"}`: "0489",
		``:    "",
		`123`: "",
	}
	for raw, want := range cases {
		if got := parseVC(json.RawMessage(raw)); got != want {
			t.Errorf("parseVC(%s) = %q, want %q", raw, got, want)
		}
	}
}
