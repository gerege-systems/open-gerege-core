// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Token service unit тест: хүчинтэй токен буцаах, хугацаа дуусахад refresh хийж
// хадгалах, refresh_token байхгүй үед Store алгасах, токен байхгүй үед алдаа.
package ssotoken

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"template/internal/business/domain"
	"template/pkg/oidc"
)

type fakeRepo struct {
	stored map[string]domain.SSOToken
}

func newFakeRepo() *fakeRepo { return &fakeRepo{stored: map[string]domain.SSOToken{}} }

func (f *fakeRepo) Upsert(_ context.Context, userID string, tok domain.SSOToken) error {
	f.stored[userID] = tok
	return nil
}

func (f *fakeRepo) Get(_ context.Context, userID string) (domain.SSOToken, error) {
	t, ok := f.stored[userID]
	if !ok {
		return domain.SSOToken{}, domain.ErrSSOTokenNotFound
	}
	return t, nil
}

// oidcTo нь refresh дуудлагыг ажиглах httptest дээр чиглэсэн oidc client.
func oidcTo(t *testing.T, h http.HandlerFunc) *oidc.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return oidc.NewClient(srv.URL, "cid", "secret", "https://app/cb", "openid offline_access")
}

func TestValidAccessTokenFresh(t *testing.T) {
	repo := newFakeRepo()
	repo.stored["u1"] = domain.SSOToken{AccessToken: "still-good", RefreshToken: "r", AccessExpiresAt: time.Now().Add(time.Hour)}
	// oidc сервер дуудагдвал тест унана (refresh хийх ёсгүй).
	oc := oidcTo(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("refresh should not be called for a fresh token")
	})
	svc := New(repo, oc)

	tok, err := svc.ValidAccessToken(context.Background(), "u1")
	if err != nil || tok != "still-good" {
		t.Fatalf("token = %q err = %v", tok, err)
	}
}

func TestValidAccessTokenRefreshes(t *testing.T) {
	repo := newFakeRepo()
	repo.stored["u1"] = domain.SSOToken{AccessToken: "expired", RefreshToken: "old-ref", AccessExpiresAt: time.Now().Add(-time.Minute)}
	oc := oidcTo(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh","refresh_token":"new-ref","expires_in":3600}`))
	})
	svc := New(repo, oc)

	tok, err := svc.ValidAccessToken(context.Background(), "u1")
	if err != nil || tok != "fresh" {
		t.Fatalf("token = %q err = %v", tok, err)
	}
	// Шинэ токенууд хадгалагдсан эсэх.
	if got := repo.stored["u1"]; got.AccessToken != "fresh" || got.RefreshToken != "new-ref" {
		t.Errorf("persisted = %+v", got)
	}
}

func TestValidAccessTokenNotFound(t *testing.T) {
	svc := New(newFakeRepo(), oidcTo(t, func(w http.ResponseWriter, r *http.Request) {}))
	_, err := svc.ValidAccessToken(context.Background(), "nobody")
	if !errors.Is(err, domain.ErrSSOTokenNotFound) {
		t.Errorf("err = %v, want ErrSSOTokenNotFound", err)
	}
}

func TestStoreSkipsWithoutRefreshToken(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, oidcTo(t, func(w http.ResponseWriter, r *http.Request) {}))
	// refresh_token хоосон — хадгалахгүй.
	if err := svc.Store(context.Background(), "u1", oidc.Tokens{AccessToken: "a", ExpiresIn: 3600}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if _, ok := repo.stored["u1"]; ok {
		t.Error("Store should skip when refresh token is empty")
	}
}

func TestStorePersistsWithRefreshToken(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, oidcTo(t, func(w http.ResponseWriter, r *http.Request) {}))
	if err := svc.Store(context.Background(), "u1", oidc.Tokens{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, ok := repo.stored["u1"]
	if !ok || got.AccessToken != "a" || got.RefreshToken != "r" {
		t.Errorf("stored = %+v ok=%v", got, ok)
	}
	if time.Until(got.AccessExpiresAt) < 30*time.Minute {
		t.Errorf("expiry too soon: %v", got.AccessExpiresAt)
	}
}
