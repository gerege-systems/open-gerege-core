// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package core нь /core/* endpoint-уудыг үйлчилнэ — Gerege Core (core.gerege.mn)
// USER FIND / ORG FIND-г wrap хийнэ. Бүгд нэвтэрсэн хэрэглэгч шаардана.
package core

import (
	"net/http"

	coreuc "template/internal/business/usecases/core"
	v1 "template/internal/http/handlers/v1"
)

type Handler struct {
	usecase coreuc.Usecase
}

func NewHandler(usecase coreuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// FindUsers godoc
// @Summary      Gerege Core хэрэглэгч хайх
// @Description  core.gerege.mn /api/user/find — search_text (core_id эсвэл регистр). Хариуг дамжуулна.
// @Tags         core
// @Produce      json
// @Param        search_text  query  string  false  "core_id / regno"
// @Success      200  {object}  v1.BaseResponse
// @Router       /core/users [get]
func (h Handler) FindUsers(w http.ResponseWriter, r *http.Request) error {
	data, err := h.usecase.FindUsers(r.Context(), r.URL.Query().Get("search_text"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "users fetched successfully", data)
}

// FindOrganizations godoc
// @Summary      Gerege Core байгууллага хайх
// @Description  core.gerege.mn /api/organization/find — search_text (регистр г.м.). Хариуг дамжуулна.
// @Tags         core
// @Produce      json
// @Param        search_text  query  string  false  "regno / нэр"
// @Success      200  {object}  v1.BaseResponse
// @Router       /core/organizations [get]
func (h Handler) FindOrganizations(w http.ResponseWriter, r *http.Request) error {
	data, err := h.usecase.FindOrganizations(r.Context(), r.URL.Query().Get("search_text"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "organizations fetched successfully", data)
}
