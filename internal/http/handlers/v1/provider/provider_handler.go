// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package provider нь OIDC provider (sso.dgov.mn = SSO) login/consent/logout-ийн
// HTTP давхарга. Next.js BFF-ийн /login, /consent, /logout хуудсууд эдгээр
// endpoint-ыг дуудаж, Hydra challenge-ыг зохицуулна. Accept endpoint-ууд нь
// нэвтэрсэн иргэнийг (session) шаардана; subject нь dan-ийн user ID.
package provider

import (
	"net/http"

	"template/internal/business/usecases/provider"
	"template/internal/http/auth"
	v1 "template/internal/http/handlers/v1"
)

type Handler struct {
	uc provider.Usecase
}

func NewHandler(uc provider.Usecase) Handler {
	return Handler{uc: uc}
}

type challengeBody struct {
	LoginChallenge   string   `json:"login_challenge,omitempty"`
	ConsentChallenge string   `json:"consent_challenge,omitempty"`
	LogoutChallenge  string   `json:"logout_challenge,omitempty"`
	GrantScope       []string `json:"grant_scope,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

type redirectResponse struct {
	RedirectTo string `json:"redirect_to"`
}

// GET /api/v1/provider/login?login_challenge=... — login хуудсанд харуулах товч.
func (h Handler) GetLogin(w http.ResponseWriter, r *http.Request) error {
	info, err := h.uc.GetLogin(r.Context(), r.URL.Query().Get("login_challenge"))
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", info)
}

// POST /api/v1/provider/login/accept — нэвтэрсэн иргэнээр login challenge-ыг
// баталгаажуулна (subject = dan user ID). Auth шаардана.
func (h Handler) AcceptLogin(w http.ResponseWriter, r *http.Request) error {
	cu, err := auth.CurrentUserFromContext(r)
	if err != nil {
		return err
	}
	var body challengeBody
	if err := v1.DecodeBody(r, &body); err != nil {
		return err
	}
	redirect, err := h.uc.AcceptLogin(r.Context(), cu.ID, body.LoginChallenge)
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", redirectResponse{RedirectTo: redirect})
}

// POST /api/v1/provider/login/reject — нэвтрэлтийг цуцална.
func (h Handler) RejectLogin(w http.ResponseWriter, r *http.Request) error {
	var body challengeBody
	if err := v1.DecodeBody(r, &body); err != nil {
		return err
	}
	redirect, err := h.uc.RejectLogin(r.Context(), body.LoginChallenge, body.Reason)
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", redirectResponse{RedirectTo: redirect})
}

// GET /api/v1/provider/consent?consent_challenge=... — consent хуудсанд харуулах.
func (h Handler) GetConsent(w http.ResponseWriter, r *http.Request) error {
	info, err := h.uc.GetConsent(r.Context(), r.URL.Query().Get("consent_challenge"))
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", info)
}

// POST /api/v1/provider/consent/accept — олгосон scope-оор consent-ыг
// баталгаажуулна. Auth шаардана (subject таарах ёстой).
func (h Handler) AcceptConsent(w http.ResponseWriter, r *http.Request) error {
	cu, err := auth.CurrentUserFromContext(r)
	if err != nil {
		return err
	}
	var body challengeBody
	if err := v1.DecodeBody(r, &body); err != nil {
		return err
	}
	redirect, err := h.uc.AcceptConsent(r.Context(), cu.ID, body.ConsentChallenge, body.GrantScope)
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", redirectResponse{RedirectTo: redirect})
}

// POST /api/v1/provider/consent/reject — зөвшөөрлийг цуцална.
func (h Handler) RejectConsent(w http.ResponseWriter, r *http.Request) error {
	var body challengeBody
	if err := v1.DecodeBody(r, &body); err != nil {
		return err
	}
	redirect, err := h.uc.RejectConsent(r.Context(), body.ConsentChallenge, body.Reason)
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", redirectResponse{RedirectTo: redirect})
}

// POST /api/v1/provider/logout/accept — RP-initiated logout-ыг баталгаажуулна.
func (h Handler) AcceptLogout(w http.ResponseWriter, r *http.Request) error {
	var body challengeBody
	if err := v1.DecodeBody(r, &body); err != nil {
		return err
	}
	redirect, err := h.uc.AcceptLogout(r.Context(), body.LogoutChallenge)
	if err != nil {
		return err
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", redirectResponse{RedirectTo: redirect})
}
