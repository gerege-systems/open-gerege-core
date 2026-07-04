// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eidprofile нь нэвтэрсэн eID хэрэглэгчийн eidmongolia.mn-ээс авах
// нэмэлт мэдээллийг (одоогоор төлөөлдөг байгууллага) харуулах handler.
package eidprofile

import (
	"net/http"

	authuc "template/internal/business/usecases/auth"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
)

type Handler struct {
	usecase authuc.Usecase
}

func NewHandler(usecase authuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// Organizations godoc
// @Summary      Төлөөлдөг байгууллагууд (eID)
// @Description  Нэвтэрсэн eID хэрэглэгчийн eidmongolia.mn-д бүртгэлтэй, төлөөлж чадах байгууллагуудыг буцаана. eID-ээр нэвтрээгүй хэрэглэгчид хоосон жагсаалт.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgRepresentationResponse}  "Representations"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      500  {object}  v1.BaseResponse  "eID provider error"
// @Router       /users/me/eid/organizations [get]
func (h Handler) Organizations(w http.ResponseWriter, r *http.Request) error {
	const funcName = "EIDOrganizations"
	ctx := r.Context()

	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}

	reps, err := h.usecase.EIDRepresentations(ctx, user.ID)
	if err != nil {
		logger.ErrorWithContext(ctx, "EIDOrganizations failed", logger.Fields{
			"controller": "eidprofile", "method": funcName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid organizations fetched",
		responses.FromEIDRepresentations(reps))
}
