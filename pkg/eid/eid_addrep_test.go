// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// AddRepresentation (eidmongolia руу төлөөлөл нэмэх) endpoint-ийн unit тест:
// POST body/path, эрхгүй (403) → ErrNotRepresentative, хариу задлалт.
package eid

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAddRepresentation(t *testing.T) {
	t.Run("posts body + parses representations", func(t *testing.T) {
		var gotPath, gotMethod, gotBody string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			_, _ = io.WriteString(w, `{"personEtsi":"PNOMN-УБ72060800","representations":[
				{"orgEtsi":"NTRMN-6235972","orgRegister":"6235972","orgName":"Гэрэгэ системс","orgNameEn":"Gerege LLC","role":"Гүйцэтгэх захирал","rightType":"ADMIN"}
			]}`)
		})
		reps, err := c.AddRepresentation(context.Background(), "PNOMN-УБ72060800", AddRepresentationInput{
			OrgRegister: "6235972", OrgName: "Гэрэгэ системс", OrgNameEn: "Gerege LLC",
			Affiliates: []OrgAffiliate{{RegNo: "уш72060800", Role: "Гүйцэтгэх захирал", Kind: "CEO"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Errorf("method = %s", gotMethod)
		}
		if !strings.HasSuffix(gotPath, "/organization/representations/etsi/PNOMN-УБ72060800") {
			t.Errorf("path = %s", gotPath)
		}
		if !strings.Contains(gotBody, `"orgRegister":"6235972"`) || !strings.Contains(gotBody, `"regNo":"уш72060800"`) || !strings.Contains(gotBody, `"kind":"CEO"`) {
			t.Errorf("body = %s", gotBody)
		}
		if len(reps) != 1 || reps[0].OrgEtsi != "NTRMN-6235972" || reps[0].RightType != "ADMIN" {
			t.Errorf("reps = %+v", reps)
		}
	})

	t.Run("403 → ErrNotRepresentative", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"эрхгүй"}`)
		})
		_, err := c.AddRepresentation(context.Background(), "PNOMN-УБ72060800", AddRepresentationInput{OrgRegister: "6235972"})
		if !errors.Is(err, ErrNotRepresentative) {
			t.Fatalf("403 → ErrNotRepresentative хүлээсэн, авсан %v", err)
		}
	})

	t.Run("empty personEtsi/orgRegister → error", func(t *testing.T) {
		c, _ := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {})
		if _, err := c.AddRepresentation(context.Background(), "  ", AddRepresentationInput{OrgRegister: "6235972"}); err == nil {
			t.Fatal("empty personEtsi should error")
		}
		if _, err := c.AddRepresentation(context.Background(), "PNOMN-X", AddRepresentationInput{}); err == nil {
			t.Fatal("empty orgRegister should error")
		}
	})
}

func TestUnlinkAndSigners(t *testing.T) {
	t.Run("RemoveRepresentation DELETE path", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			_, _ = io.WriteString(w, `{"representations":[]}`)
		})
		reps, err := c.RemoveRepresentation(context.Background(), "PNOMN-УБ72060800", "6235972")
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete || !strings.HasSuffix(gotPath, "/organization/representations/etsi/PNOMN-УБ72060800/6235972") {
			t.Errorf("%s %s", gotMethod, gotPath)
		}
		if len(reps) != 0 {
			t.Errorf("reps = %d", len(reps))
		}
	})

	t.Run("AddSigner POST → PENDING + pendingConfirmation", func(t *testing.T) {
		var gotPath, gotMethod, gotBody string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			_, _ = io.WriteString(w, `{"orgRegister":"6235972","signers":[{"personEtsi":"PNOMN-МА74101813","regNo":"ма74101813","name":"Цэнддорж Эрдэнэбат","role":"Нягтлан бодогч","rightType":"MANAGER","status":"PENDING","source":"MANUAL","self":false}],"pendingConfirmation":{"orgRegister":"6235972","orgName":"Гэрэгэ системс","signerEtsi":"PNOMN-МА74101813","signerRegNo":"ма74101813","sessionId":"sess-1"}}`)
		})
		res, err := c.AddSigner(context.Background(), "6235972", "PNOMN-УБ72060800", AddSignerInput{SignerRegNo: "ма74101813", Role: "Нягтлан бодогч"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || !strings.HasSuffix(gotPath, "/organization/signers/6235972/etsi/PNOMN-УБ72060800") {
			t.Errorf("%s %s", gotMethod, gotPath)
		}
		if !strings.Contains(gotBody, `"signerRegNo":"ма74101813"`) {
			t.Errorf("body = %s", gotBody)
		}
		if len(res.Signers) != 1 || res.Signers[0].Role != "Нягтлан бодогч" || res.Signers[0].RightType != "MANAGER" || res.Signers[0].Status != "PENDING" {
			t.Errorf("signers = %+v", res.Signers)
		}
		if res.PendingConfirmation == nil || res.PendingConfirmation.SessionID != "sess-1" || res.PendingConfirmation.SignerEtsi != "PNOMN-МА74101813" {
			t.Errorf("pendingConfirmation = %+v", res.PendingConfirmation)
		}
	})

	t.Run("OrgSigners 403 → ErrNotRepresentative", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
		if _, err := c.OrgSigners(context.Background(), "6235972", "PNOMN-X"); !errors.Is(err, ErrNotRepresentative) {
			t.Fatalf("403 → ErrNotRepresentative, авсан %v", err)
		}
	})

	t.Run("AddSigner 404 → ErrSignerNotEnrolled", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		if _, err := c.AddSigner(context.Background(), "6235972", "PNOMN-X", AddSignerInput{SignerRegNo: "яя99999999"}); !errors.Is(err, ErrSignerNotEnrolled) {
			t.Fatalf("404 → ErrSignerNotEnrolled, авсан %v", err)
		}
	})

	t.Run("RemoveSigner DELETE ?signer=", func(t *testing.T) {
		var gotPath, gotQuery, gotMethod string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery, gotMethod = r.URL.Path, r.URL.RawQuery, r.Method
			_, _ = io.WriteString(w, `{"orgRegister":"6235972","signers":[]}`)
		})
		if _, err := c.RemoveSigner(context.Background(), "6235972", "PNOMN-УБ72060800", "ма74101813"); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete || !strings.HasSuffix(gotPath, "/organization/signers/6235972/etsi/PNOMN-УБ72060800") || !strings.Contains(gotQuery, "signer=") {
			t.Errorf("%s %s?%s", gotMethod, gotPath, gotQuery)
		}
	})
}
