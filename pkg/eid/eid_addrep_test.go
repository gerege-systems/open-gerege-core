// Gerege Template Version 27.0
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
				{"orgEtsi":"NTRMN-6235972","orgRegister":"6235972","orgName":"Гэрэгэ системс","orgNameEn":"Gerege LLC","role":"Гүйцэтгэх захирал","rightType":"SOLE"}
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
		if len(reps) != 1 || reps[0].OrgEtsi != "NTRMN-6235972" || reps[0].RightType != "SOLE" {
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
