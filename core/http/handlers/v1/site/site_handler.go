// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package site нь сайтын нийтийн харагдацын default-ыг үйлчилнэ — landing
// зэрэг нийтийн хуудсанд уншигдах GET (auth-гүй) болон админ засварлах PUT
// ('settings.manage').
package site

import (
	"net/http"

	"github.com/gerege-systems/public-gerege-core/core/business/domain"
	siteuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/site"
	"github.com/gerege-systems/public-gerege-core/core/http/datatransfers/requests"
	"github.com/gerege-systems/public-gerege-core/core/http/datatransfers/responses"
	v1 "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/public-gerege-core/pkg/validators"
)

// AuthSurface нь нэвтрэх гадаргууны боот үеийн тохиргоо. Утгыг нь server.go
// config-оос бүрдүүлж дамжуулна — handler давхарга config-оос ХАМААРАХГҮЙ
// (энэ багцын бусад handler мөн адил).
type AuthSurface struct {
	// Mode — "provider" (платформ өөрөө нэвтрүүлнэ) эсвэл "client" (дээд SSO).
	Mode string
	// SSOIssuer — client горимд дээд IdP-ийн нийтийн issuer.
	SSOIssuer string
	// Provider — платформ өөрөө OIDC issuer мөн эсэх.
	Provider bool
}

type Handler struct {
	usecase siteuc.Usecase
	auth    AuthSurface
}

func NewHandler(usecase siteuc.Usecase, auth AuthSurface) Handler {
	return Handler{usecase: usecase, auth: auth}
}

// GetAppearance godoc
// @Summary Сайтын харагдацын default-ыг унших
// @Description Landing болон нийтийн хуудсанд хэрэглэх нийтийн харагдац (accent · font · style · theme). Нэвтрэлт шаардахгүй.
// @Tags site
// @Produce json
// @Success 200 {object} v1.BaseResponse{data=responses.SiteAppearanceResponse}
// @Router /site/appearance [get]
func (h Handler) GetAppearance(w http.ResponseWriter, r *http.Request) error {
	a, err := h.usecase.GetAppearance(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "appearance fetched", responses.ToSiteAppearance(a))
}

// GetAuth godoc
// @Summary Нэвтрэх гадаргууны горимыг унших
// @Description Нүүр хуудас болон /login нь нэвтрэх картыг өөрөө үзүүлэх ('provider') үү, эсвэл дээд SSO руу шилжүүлэх ('client') үү. Нэвтрэлт шаардахгүй.
// @Tags site
// @Produce json
// @Success 200 {object} v1.BaseResponse{data=responses.SiteAuthResponse}
// @Router /site/auth [get]
func (h Handler) GetAuth(w http.ResponseWriter, r *http.Request) error {
	return v1.NewSuccessResponse(w, r, http.StatusOK, "auth mode fetched", responses.SiteAuthResponse{
		Mode:      h.auth.Mode,
		SSOIssuer: h.auth.SSOIssuer,
		Provider:  h.auth.Provider,
	})
}

// SetAppearance godoc
// @Summary Сайтын харагдацын default-ыг шинэчлэх
// @Description Админ (settings.manage) сайтын нийтийн харагдацыг өөрчилнө. accent нь preset нэр эсвэл '#rrggbb' hex.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body requests.SiteAppearanceUpdateRequest true "Харагдацын шинэ утга"
// @Success 200 {object} v1.BaseResponse
// @Failure 400 {object} v1.BaseResponse "Буруу утга"
// @Failure 403 {object} v1.BaseResponse "settings.manage эрх дутуу"
// @Router /site/appearance [put]
func (h Handler) SetAppearance(w http.ResponseWriter, r *http.Request) error {
	var req requests.SiteAppearanceUpdateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	err := h.usecase.SetAppearance(r.Context(), domain.SiteAppearance{
		Accent: req.Accent,
		Font:   req.Font,
		Style:  req.Style,
		Theme:  req.Theme,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "appearance updated", nil)
}
