// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModuleForPath(t *testing.T) {
	r := Builtin()
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"/api/v1/gov/services", "gov", true},
		{"/api/v1/gov", "gov", true},
		{"/api/v1/auth/eid/start", "auth", true},
		// Урт угтвар богиныг ялна: auth-ийн дэд зам ч superadmin-ийх.
		{"/api/v1/auth/superadmin/mfa", "superadmin", true},
		// admin/ai нь ai-д, үлдсэн admin нь users-т.
		{"/api/v1/admin/ai/prompts", "ai", true},
		{"/api/v1/admin/users", "users", true},
		{"/userinfo", "provider", true},
		{"/.well-known/openid-configuration", "provider", true},
		// Модульд харьяалагдаагүй замууд.
		{"/api/", "", false},
		{"/metrics", "", false},
		{"/api/v1/userinfo-x", "", false}, // "/userinfo" нь /userinfo-x-д таарахгүй
	}
	for _, tc := range cases {
		id, ok := r.ModuleForPath(tc.path)
		if ok != tc.ok || id != tc.want {
			t.Errorf("%s: (%q,%v) гарлаа, (%q,%v) хүлээсэн", tc.path, id, ok, tc.want, tc.ok)
		}
	}
}

func TestGateBlocksDisabledModule(t *testing.T) {
	reg := mustReg(t,
		Manifest{ID: "auth", Kind: KindCore, RoutePrefixes: []string{"/api/v1/auth/"}},
		Manifest{ID: "gspace", Kind: KindBusiness, RoutePrefixes: []string{"/api/v1/gspace/"}},
	)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := Gate(reg)(next)

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	// Идэвхтэй үед нэвтэрнэ.
	if rec := get("/api/v1/gspace/"); rec.Code != http.StatusOK {
		t.Fatalf("идэвхтэй модуль: %d", rec.Code)
	}
	// Унтраасны дараа 404 + JSON.
	if err := reg.Disable("gspace"); err != nil {
		t.Fatal(err)
	}
	rec := get("/api/v1/gspace/upload")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("унтарсан модуль: %d, 404 хүлээсэн", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type: %s", ct)
	}
	if !strings.Contains(rec.Body.String(), `"status":false`) {
		t.Fatalf("body: %s", rec.Body.String())
	}
	// Бусад модулийн зам, эзэнгүй зам хэвээр нэвтэрнэ.
	if rec := get("/api/v1/auth/eid/start"); rec.Code != http.StatusOK {
		t.Fatalf("core модуль: %d", rec.Code)
	}
	if rec := get("/api/"); rec.Code != http.StatusOK {
		t.Fatalf("эзэнгүй зам: %d", rec.Code)
	}
}
