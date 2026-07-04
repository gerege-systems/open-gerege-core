// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"template/internal/constants"
	"template/internal/http/middlewares"
	"template/internal/test/mocks"
	"template/pkg/jwt"
)

var (
	jwtService          jwt.JWTService
	authBasicMiddleware func(http.Handler) http.Handler
	authAdminMiddleware func(http.Handler) http.Handler
)

const (
	adminEndpoint = "/admin"
	forEveryone   = "/everyone"
)

// authenticatedHandler нь auth middleware амжилттай дамжуулсны дараа
// дуудагдах доод урсгалын handler. context-д тавьсан claim-г уншиж,
// баталгаажуулагдсан хариу буцаана.
func authenticatedHandler(w http.ResponseWriter, r *http.Request) {
	// Амжилтын урсгал дээр claim context-д тавигдсан байх ёстой.
	if _, ok := r.Context().Value(constants.CtxAuthenticatedUserKey).(jwt.JwtCustomClaim); !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  true,
		"message": "nice to meet you again sir...",
	})
}

func setup(t *testing.T) {
	t.Helper()
	jwtService = jwt.NewJWTService("test-secret-key", "test-issuer", 5)
	authBasicMiddleware = middlewares.NewAuthMiddleware(jwtService, nil, false)
	authAdminMiddleware = middlewares.NewAuthMiddleware(jwtService, nil, true)
}

// serve нь сонгосон middleware-ийг authenticatedHandler-т ороож хүсэлтийг
// гүйцэтгэнэ. middleware алдаа дээр гинжийг таслаж 401 бичдэг тул
// authenticatedHandler дуудагдахгүй.
func serve(mw func(http.Handler) http.Handler, r *http.Request) *httptest.ResponseRecorder {
	h := mw(http.HandlerFunc(authenticatedHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func generateToken(isAdmin bool) (token string, err error) {
	token, err = jwtService.GenerateToken("ddfcea5c-d919-4a8f-a631-4ace39337s3a", isAdmin, 2, "najibfikri13@gmail.com")
	return
}

func getAdminToken() (string, error) {
	return generateToken(true)
}

func getBasicToken() (string, error) {
	return generateToken(false)
}

func TestAuthMiddleware(t *testing.T) {
	setup(t)

	t.Run("Test 1 | Success Get Admin Handler", func(t *testing.T) {
		token, err := getAdminToken()
		if err != nil {
			t.Error(err)
		}

		r := httptest.NewRequest(http.MethodGet, adminEndpoint, http.NoBody)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		rec := serve(authAdminMiddleware, r)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, rec.Body.String(), "nice to meet you again sir")
	})
	t.Run("Test 2 | Invalid Token", func(t *testing.T) {
		token := "mwehehe"

		r := httptest.NewRequest(http.MethodGet, forEveryone, http.NoBody)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		rec := serve(authBasicMiddleware, r)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, rec.Body.String(), "invalid token")
	})
	t.Run("Test 3 | Must Content Bearer", func(t *testing.T) {
		token, err := getBasicToken()
		if err != nil {
			t.Error(err)
		}

		r := httptest.NewRequest(http.MethodGet, forEveryone, http.NoBody)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", fmt.Sprintf("Token %s", token))

		rec := serve(authBasicMiddleware, r)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, rec.Body.String(), "token must content bearer")
	})
	t.Run("Test 4 | Invalid Format", func(t *testing.T) {
		token, err := getBasicToken()
		if err != nil {
			t.Error(err)
		}

		r := httptest.NewRequest(http.MethodGet, forEveryone, http.NoBody)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", fmt.Sprintf("Bearer token: %s", token))

		rec := serve(authBasicMiddleware, r)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, rec.Body.String(), "invalid header format")
	})
	t.Run("Test 5 | Not Authorize", func(t *testing.T) {
		token, err := getBasicToken()
		if err != nil {
			t.Error(err)
		}

		r := httptest.NewRequest(http.MethodGet, adminEndpoint, http.NoBody)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		rec := serve(authAdminMiddleware, r)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, rec.Body.String(), "you don't have access for this action")
	})
}

// TestAuthMiddlewareRevocationFailClosed нь revocation шалгалт (logout deny-list
// / нууц үг солилтын cutoff) Redis-ийн жинхэнэ алдаанд FAIL-CLOSED (503) байж,
// харин key байхгүй (redis.Nil miss) үед токеныг нэвтрүүлдгийг баталгаажуулна.
func TestAuthMiddlewareRevocationFailClosed(t *testing.T) {
	setup(t)

	newTokenReq := func(t *testing.T) *http.Request {
		t.Helper()
		token, err := getBasicToken()
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet, forEveryone, http.NoBody)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		return r
	}

	t.Run("Redis unavailable → fail-closed 503", func(t *testing.T) {
		cache := &mocks.RedisCache{}
		cache.On("Get", mock.Anything, mock.Anything).Return("", errors.New("dial tcp: connection refused"))
		mw := middlewares.NewAuthMiddleware(jwtService, cache, false)

		rec := serve(mw, newTokenReq(t))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "session verification temporarily unavailable")
	})

	t.Run("Redis miss (redis.Nil) → allow 200", func(t *testing.T) {
		cache := &mocks.RedisCache{}
		cache.On("Get", mock.Anything, mock.Anything).Return("", redis.Nil)
		mw := middlewares.NewAuthMiddleware(jwtService, cache, false)

		rec := serve(mw, newTokenReq(t))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "nice to meet you again sir")
	})
}
