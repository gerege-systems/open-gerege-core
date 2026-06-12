// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"template/internal/apperror"
	"template/internal/business/domain"
	usersuc "template/internal/business/usecases/users"
	"template/internal/constants"
	v1 "template/internal/http/handlers/v1"
	usershandler "template/internal/http/handlers/v1/users"
	"template/internal/test/mocks"
	jwtpkg "template/pkg/jwt"
)

// injectClaims нь auth middleware-ийн оронд баталгаажуулагдсан claim-г
// хүсэлтийн context-д тавьдаг тестийн middleware.
func injectClaims(userID, email string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), constants.CtxAuthenticatedUserKey, jwtpkg.JwtCustomClaim{
				UserID: userID, Email: email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TestGetUserDataHandler(t *testing.T) {
	// build нь claim тарьсан /me route-той mux буцаана.
	build := func(t *testing.T) (*mocks.UsersUsecase, http.Handler) {
		uc := mocks.NewUsersUsecase(t)
		h := usershandler.NewHandler(uc)
		mux := http.NewServeMux()
		mux.Handle("GET /me", injectClaims("user-1", "patrick@example.com")(v1.Wrap(h.GetUserData)))
		return uc, mux
	}

	t.Run("happy path returns user data", func(t *testing.T) {
		uc, r := build(t)
		uc.On("GetByID", mock.Anything, usersuc.GetByIDRequest{ID: "user-1"}).
			Return(usersuc.GetByIDResponse{User: domain.User{ID: "user-1", Email: "patrick@example.com", Username: "patrick"}}, nil).Once()

		req := httptest.NewRequest("GET", "/me", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "patrick")
	})

	t.Run("usecase NotFound surfaces as 404", func(t *testing.T) {
		uc, r := build(t)
		uc.On("GetByID", mock.Anything, usersuc.GetByIDRequest{ID: "user-1"}).
			Return(usersuc.GetByIDResponse{}, apperror.NotFound("user not found")).Once()

		req := httptest.NewRequest("GET", "/me", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("missing claims returns 401", func(t *testing.T) {
		uc := mocks.NewUsersUsecase(t)
		h := usershandler.NewHandler(uc)
		// claim тарихгүйгээр шууд handler-г холбоно — CurrentUserFromContext
		// танигдах claim олохгүй тул handler нь 401 буцаах ёстой.
		mux := http.NewServeMux()
		mux.Handle("GET /me", v1.Wrap(h.GetUserData))

		req := httptest.NewRequest("GET", "/me", http.NoBody)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
