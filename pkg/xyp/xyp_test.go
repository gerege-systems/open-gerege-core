// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package xyp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	t.Run("parses organization + basic auth", func(t *testing.T) {
		var gotPath, gotAuth, gotBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			_, _ = io.WriteString(w, `{"found":true,"organization":{"reg_no":"6235972","name":"Гэрэгэ системс","ceo_reg_no":"уш72060800","founders":[{"name":"нацагдорж","reg_no":"уш72060800","type":"Иргэн","share_percent":"30"}],"stake_holders":[{"name":"цэнддорж","reg_no":"ма74101813","position":"ТУЗ-ийн дарга"}]}}`)
		}))
		defer srv.Close()

		c := NewClient(srv.URL, "vfy_id", "secret")
		org, err := c.Lookup(context.Background(), "6235972")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(gotPath, "/v1/org/lookup") {
			t.Errorf("path = %s", gotPath)
		}
		if !strings.HasPrefix(gotAuth, "Basic ") {
			t.Errorf("auth = %s (Basic хүлээсэн)", gotAuth)
		}
		if !strings.Contains(gotBody, `"reg_no":"6235972"`) {
			t.Errorf("body = %s", gotBody)
		}
		if org.Name != "Гэрэгэ системс" || org.CEORegNo != "уш72060800" {
			t.Errorf("org = %+v", org)
		}
		if len(org.Founders) != 1 || org.Founders[0].RegNo != "уш72060800" {
			t.Errorf("founders = %+v", org.Founders)
		}
		if len(org.StakeHolders) != 1 || org.StakeHolders[0].RegNo != "ма74101813" {
			t.Errorf("stakeholders = %+v", org.StakeHolders)
		}
	})

	t.Run("found=false → ErrNotFound", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"found":false}`)
		}))
		defer srv.Close()
		c := NewClient(srv.URL, "id", "secret")
		if _, err := c.Lookup(context.Background(), "0000000"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("found=false → ErrNotFound хүлээсэн, авсан %v", err)
		}
	})

	t.Run("404 → ErrNotFound", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"not found"}`)
		}))
		defer srv.Close()
		c := NewClient(srv.URL, "id", "secret")
		if _, err := c.Lookup(context.Background(), "0000000"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("404 → ErrNotFound хүлээсэн, авсан %v", err)
		}
	})

	t.Run("креденшлгүй → ErrNotConfigured", func(t *testing.T) {
		c := NewClient("", "", "")
		if _, err := c.Lookup(context.Background(), "6235972"); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("ErrNotConfigured хүлээсэн, авсан %v", err)
		}
	})

	t.Run("хоосон reg_no → алдаа", func(t *testing.T) {
		c := NewClient("", "id", "secret")
		if _, err := c.Lookup(context.Background(), "  "); err == nil {
			t.Fatal("хоосон reg_no алдаа буцаах ёстой")
		}
	})

	t.Run("401 → алдаа", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		c := NewClient(srv.URL, "id", "bad")
		if _, err := c.Lookup(context.Background(), "6235972"); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("401 → энгийн алдаа хүлээсэн, авсан %v", err)
		}
	})
}
