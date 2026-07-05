// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package sso нь /sso/* endpoint-уудыг үйлчилнэ — Gerege SSO (sso.gerege.mn,
// OIDC) нэвтрэлтийн 2 дахь урсгал. Start нь authorize URL буцаана, Callback нь
// code-ийг солиж токен олгоно. Бүгд нэвтрэхээс өмнөх (ServiceRLSContext) урсгал.
package sso

import (
	"net/http"

	ssouc "template/internal/business/usecases/sso"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

type Handler struct {
	usecase ssouc.Usecase
}

func NewHandler(usecase ssouc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// startResponse нь browser-ийг чиглүүлэх SSO authorize URL.
type startResponse struct {
	AuthURL string `json:"auth_url"`
}

// callbackRequest нь BFF-ээс ирэх callback параметрүүд.
type callbackRequest struct {
	State string `json:"state"`
	Code  string `json:"code"`
}

// callbackResponse нь токен хос + нууц БУС хэрэглэгчийн талбарууд. BFF нь
// token/refresh_token-ийг httpOnly cookie-д суулгаж, browser руу гаргахгүй.
type callbackResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	SSOLogoutRef string `json:"sso_logout_ref"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
}

// nativeResponse нь mobile (PKCE) урсгалын token хос + нууц БУС хэрэглэгч. BFF
// нь token/refresh_token-ийг httpOnly cookie-д суулгаж, browser руу гаргахгүй.
type nativeResponse struct {
	Token        string                 `json:"token"`
	RefreshToken string                 `json:"refresh_token"`
	SSOLogoutRef string                 `json:"sso_logout_ref,omitempty"`
	User         responses.UserResponse `json:"user"`
}

// logoutRequest нь BFF-ээс ирэх logout ref (callback-д олгосон).
type logoutRequest struct {
	Ref string `json:"ref"`
}

// logoutResponse нь SSO дээр session дуусгах RP-initiated logout URL.
type logoutResponse struct {
	SSOLogoutURL string `json:"sso_logout_url"`
}

// Start godoc
// @Summary      Gerege SSO нэвтрэлт эхлүүлэх
// @Description  sso.gerege.mn (OIDC) authorize URL-ийг state-тэй буцаана. BFF browser-ийг тийш чиглүүлнэ.
// @Tags         sso
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /sso/start [post]
func (h Handler) Start(w http.ResponseWriter, r *http.Request) error {
	res, err := h.usecase.Start(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "sso started", startResponse{AuthURL: res.AuthURL})
}

// Callback godoc
// @Summary      Gerege SSO callback
// @Description  authorize callback-ийн state+code-ийг шалгаж, code-ийг токен болгож солин, иргэнийг sso_sub-ээр upsert хийж, JWT хос олгоно.
// @Tags         sso
// @Accept       json
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /sso/callback [post]
func (h Handler) Callback(w http.ResponseWriter, r *http.Request) error {
	var req callbackRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	res, err := h.usecase.Complete(r.Context(), req.State, req.Code)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "sso login complete", callbackResponse{
		Token:        res.Token,
		RefreshToken: res.RefreshToken,
		SSOLogoutRef: res.LogoutRef,
		UserID:       res.User.ID,
		Username:     res.User.Username,
	})
}

// SSONative godoc
// @Summary      Gerege SSO native (mobile PKCE) нэвтрэлт
// @Description  Mobile app (iOS/Android) ASWebAuthenticationSession-ийн PKCE code-ийг public client-ээр (client_secret-гүй, code_verifier-тэй) солин, иргэнийг upsert хийж JWT хос олгоно. State шалгалтгүй (PKCE хамгаална). BFF нь token/refresh_token-ийг httpOnly cookie-д суулгана.
// @Tags         sso
// @Accept       json
// @Produce      json
// @Param        request  body      requests.SSONativeRequest  true  "Native PKCE code exchange"
// @Success      200      {object}  v1.BaseResponse
// @Failure      400      {object}  v1.BaseResponse  "Missing code/code_verifier"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Router       /sso/native [post]
func (h Handler) SSONative(w http.ResponseWriter, r *http.Request) error {
	var req requests.SSONativeRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	res, err := h.usecase.CompleteNative(r.Context(), req.Code, req.CodeVerifier, req.RedirectURI)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "sso native login complete", nativeResponse{
		Token:        res.Token,
		RefreshToken: res.RefreshToken,
		SSOLogoutRef: res.LogoutRef,
		User:         responses.FromV1Domain(res.User),
	})
}

// Logout godoc
// @Summary      Gerege SSO logout URL
// @Description  logout ref-ээр (callback-д олгосон) SSO (Hydra) end_session_endpoint URL-ийг байгуулна. BFF browser-ийг тийш чиглүүлж SSO дээрх session-ийг дуусгана.
// @Tags         sso
// @Accept       json
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /sso/logout [post]
func (h Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	var req logoutRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	url, err := h.usecase.LogoutURL(r.Context(), req.Ref)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "sso logout url", logoutResponse{SSOLogoutURL: url})
}
