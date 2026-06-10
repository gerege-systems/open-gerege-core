// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	authuc "template/internal/business/usecases/auth"
	"template/internal/constants"
	v1 "template/internal/http/handlers/v1"
	authhandler "template/internal/http/handlers/v1/auth"
	"template/internal/test/mocks"
	jwtpkg "template/pkg/jwt"
	"template/pkg/validators"
)

func init() {
	_ = validators.ValidatePayloads(struct{}{})
}

type authHarness struct {
	uc     *mocks.AuthUsecase
	router http.Handler
}

func newAuthHarness(t *testing.T) authHarness {
	t.Helper()
	uc := mocks.NewAuthUsecase(t)
	h := authhandler.NewHandler(uc)

	// chi/net-http port: handler-ууд алдаа буцаадаг тул v1.Wrap нь тэдгээрийг
	// production сервертэй ижил төвлөрсөн дугтуйгаар стандарт
	// http.HandlerFunc болгоно. RespondWithError/NewErrorResponse нь handler
	// дотроос статус буулгалтыг хийдэг тул ErrorHandler hook шаардлагагүй.
	mux := http.NewServeMux()
	mux.Handle("POST /login", v1.Wrap(h.Login))
	mux.Handle("POST /register", v1.Wrap(h.Register))
	mux.Handle("POST /password/forgot", v1.Wrap(h.ForgotPassword))
	mux.Handle("POST /password/reset", v1.Wrap(h.ResetPassword))
	mux.Handle("PUT /password/change",
		injectClaims("user-1", "patrick@example.com")(v1.Wrap(h.ChangePassword)))

	return authHarness{uc: uc, router: mux}
}

// injectClaims нь auth middleware-ийн оронд баталгаажуулагдсан claim-г
// хүсэлтийн context-д тавьдаг тестийн middleware. Production-д үүнийг
// NewAuthMiddleware хийдэг.
func injectClaims(userID, email string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), constants.CtxAuthenticatedUserKey, jwtpkg.JwtCustomClaim{
				UserID: userID,
				Email:  email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func doJSON(t *testing.T, h authHarness, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestLoginHandler(t *testing.T) {
	t.Run("happy path returns 200 and tokens", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("Login", mock.Anything, authuc.LoginRequest{Email: "patrick@example.com", Password: "Pwd_123!"}).
			Return(authuc.LoginResponse{
				AccessToken: "access-tok", RefreshToken: "refresh-tok",
			}, nil).Once()

		rec := doJSON(t, h, "POST", "/login", map[string]string{
			"email": "patrick@example.com", "password": "Pwd_123!",
		})
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "access-tok")
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		h := newAuthHarness(t)
		req := httptest.NewRequest("POST", "/login", bytes.NewBufferString("{bad json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("validation failure returns 422", func(t *testing.T) {
		h := newAuthHarness(t)
		rec := doJSON(t, h, "POST", "/login", map[string]string{
			"email": "not-an-email", "password": "p",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("usecase Unauthorized returns 401", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("Login", mock.Anything, authuc.LoginRequest{Email: "x@y.com", Password: "wrong"}).
			Return(authuc.LoginResponse{}, apperror.Unauthorized("invalid email or password")).Once()
		rec := doJSON(t, h, "POST", "/login", map[string]string{
			"email": "x@y.com", "password": "wrong",
		})
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestForgotPasswordHandler(t *testing.T) {
	t.Run("happy path returns 200", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("ForgotPassword", mock.Anything, authuc.ForgotPasswordRequest{Email: "patrick@example.com"}).Return(nil).Once()
		rec := doJSON(t, h, "POST", "/password/forgot", map[string]string{"email": "patrick@example.com"})
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("infra error from usecase still returns non-2xx", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("ForgotPassword", mock.Anything, authuc.ForgotPasswordRequest{Email: "patrick@example.com"}).
			Return(apperror.InternalCause(assertErr("redis down"))).Once()
		rec := doJSON(t, h, "POST", "/password/forgot", map[string]string{"email": "patrick@example.com"})
		assert.GreaterOrEqual(t, rec.Code, 500)
	})
}

func TestResetPasswordHandler(t *testing.T) {
	t.Run("happy path returns 200", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("ResetPassword", mock.Anything, authuc.ResetPasswordRequest{Email: "patrick@example.com", Code: "123456", NewPassword: "Newpwd_9999!"}).Return(nil).Once()
		rec := doJSON(t, h, "POST", "/password/reset", map[string]string{
			"email": "patrick@example.com", "code": "123456", "new_password": "Newpwd_9999!",
		})
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("invalid code returns 401", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("ResetPassword", mock.Anything, authuc.ResetPasswordRequest{Email: "patrick@example.com", Code: "999999", NewPassword: "Newpwd_9999!"}).
			Return(apperror.Unauthorized("reset code is invalid or expired")).Once()
		rec := doJSON(t, h, "POST", "/password/reset", map[string]string{
			"email": "patrick@example.com", "code": "999999", "new_password": "Newpwd_9999!",
		})
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestChangePasswordHandler(t *testing.T) {
	t.Run("happy path returns 200 with claims injected", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("ChangePassword", mock.Anything, authuc.ChangePasswordRequest{
			UserID:          "user-1",
			CurrentPassword: "Pwd_123!",
			NewPassword:     "Newpwd_9999!",
		}).Return(nil).Once()
		rec := doJSON(t, h, "PUT", "/password/change", map[string]string{
			"current_password": "Pwd_123!", "new_password": "Newpwd_9999!",
		})
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("usecase Unauthorized when current password wrong", func(t *testing.T) {
		h := newAuthHarness(t)
		h.uc.On("ChangePassword", mock.Anything, authuc.ChangePasswordRequest{
			UserID:          "user-1",
			CurrentPassword: "wrong",
			NewPassword:     "Newpwd_9999!",
		}).Return(apperror.Unauthorized("current password is incorrect")).Once()
		rec := doJSON(t, h, "PUT", "/password/change", map[string]string{
			"current_password": "wrong", "new_password": "Newpwd_9999!",
		})
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func assertErr(s string) error { return &simpleErr{msg: s} }

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }
