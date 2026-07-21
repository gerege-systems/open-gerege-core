// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package superadmin

import (
	"net/http"
	"net/url"

	superadminuc "template/internal/business/usecases/superadmin"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

// ListInvites godoc
// @Summary      Super admin урилгуудыг жагсаах
// @Description  Super admin болох урилгуудыг (хүлээгдэж буй + ашигласан) буцаана. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=[]responses.SuperadminInviteResponse}  "Invites fetched"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Router       /v1/superadmin/invites [get]
func (h Handler) ListInvites(w http.ResponseWriter, r *http.Request) error {
	res, err := h.usecase.ListInvites(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "invites fetched successfully", responses.ToSuperadminInviteList(res.Invites))
}

// CreateInvite godoc
// @Summary      Super admin урих
// @Description  И-мэйлийг super admin болох allow-list-д нэмнэ. Урилга нь эрхийг ШУУД олгодоггүй — урьсан хүн /auth/superadmin/onboard шидтэнг (Google + eID + и-мэйл OTP + TOTP) бүрэн давж байж super admin болно. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.SuperadminInviteRequest  true  "Invite email"
// @Success      201  {object}  v1.BaseResponse{data=responses.SuperadminInviteResponse}  "Invite created"
// @Failure      400  {object}  v1.BaseResponse  "Invalid email"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Failure      409  {object}  v1.BaseResponse  "Email already invited"
// @Router       /v1/superadmin/invites [post]
func (h Handler) CreateInvite(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	var req requests.SuperadminInviteRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	res, err := h.usecase.CreateInvite(r.Context(), superadminuc.CreateInviteRequest{
		Email: req.Email, ActorEmail: user.Email,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "invite created successfully", responses.FromSuperadminInvite(res.Invite))
}

// DeleteInvite godoc
// @Summary      Super admin урилгыг цуцлах
// @Description  Урилгыг allow-list-ээс хасна — хараахан бүртгүүлээгүй бол цаашид бүртгүүлэх боломжгүй болно. Зөвхөн super admin хандана.
// @Tags         superadmin
// @Produce      json
// @Security     BearerAuth
// @Param        email  path      string  true  "Invited email (URL-encoded)"
// @Success      200  {object}  v1.BaseResponse  "Invite deleted"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not a super admin"
// @Failure      404  {object}  v1.BaseResponse  "Invite not found"
// @Router       /v1/superadmin/invites/{email} [delete]
func (h Handler) DeleteInvite(w http.ResponseWriter, r *http.Request) error {
	// И-мэйл нь URL-д "@"/"." агуулдаг тул клиент URL-encode хийж илгээнэ.
	raw := chi.URLParam(r, "email")
	email, decErr := url.PathUnescape(raw)
	if decErr != nil {
		email = raw
	}
	if err := h.usecase.DeleteInvite(r.Context(), superadminuc.DeleteInviteRequest{Email: email}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "invite deleted successfully", nil)
}
