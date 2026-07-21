// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Representations (төлөөлдөг байгууллага) endpoint-ийн unit тест: wire задлалт,
// 404 → хоосон slice (иргэн байгууллага төлөөлдөггүй), path-д personEtsi зөв
// орох.
package eid

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepresentations(t *testing.T) {
	t.Run("parses representations", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = io.WriteString(w, `{"personEtsi":"PNOMN-УБ99887766","representations":[
				{"orgEtsi":"NTRMN-1234567","orgRegister":"1234567","orgName":"Тест ХХК","orgNameEn":"Test LLC","role":"Захирал","rightType":"ADMIN"}
			]}`)
		})
		reps, err := c.Representations(context.Background(), "PNOMN-УБ99887766")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(gotPath, "/organization/representations/etsi/PNOMN-УБ99887766") {
			t.Errorf("path = %s", gotPath)
		}
		if len(reps) != 1 {
			t.Fatalf("reps = %d", len(reps))
		}
		if reps[0].OrgEtsi != "NTRMN-1234567" || reps[0].OrgName != "Тест ХХК" || reps[0].RightType != "ADMIN" || reps[0].Role != "Захирал" {
			t.Errorf("rep = %+v", reps[0])
		}
	})

	t.Run("404 → empty (represents no org)", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"not found"}`)
		})
		reps, err := c.Representations(context.Background(), "PNOMN-УБ00000000")
		if err != nil {
			t.Fatal(err)
		}
		if len(reps) != 0 {
			t.Errorf("404 → хоосон байх ёстой, авсан %d", len(reps))
		}
	})

	t.Run("empty personEtsi is an error", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {})
		if _, err := c.Representations(context.Background(), "  "); err == nil {
			t.Fatal("empty personEtsi should error")
		}
	})

	t.Run("server error surfaces", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		t.Cleanup(srv.Close)
		c := NewClient(srv.URL, testUUID, testName, testSecret, "ADVANCED")
		if _, err := c.Representations(context.Background(), "PNOMN-X"); err == nil {
			t.Fatal("5xx should error")
		}
	})
}
