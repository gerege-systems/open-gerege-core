// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eidprofile нь нэвтэрсэн eID хэрэглэгчийн eidmongolia.mn-ээс авах
// нэмэлт мэдээллийг (одоогоор төлөөлдөг байгууллага) харуулах handler.
package eidprofile

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	authuc "template/internal/business/usecases/auth"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/logger"
	"template/pkg/validators"
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

// AddOrganization godoc
// @Summary      Байгууллага холбох (eID)
// @Description  Улсын бүртгэлээс (XYP) байгууллагыг регистрийн дугаараар хайж, нэвтэрсэн иргэнийг (eID РД нь тухайн байгууллагын захирал/үүсгэн байгуулагч/хувь эзэмшигч бол) eidmongolia.mn-д төлөөлөл болгон холбоно. Иргэний бүх төлөөлдөг байгууллагыг буцаана.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.EIDOrgRegisterRequest  true  "Байгууллагын регистрийн дугаар"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgRepresentationResponse}  "Representations"
// @Failure      400  {object}  v1.BaseResponse  "Invalid body"
// @Failure      403  {object}  v1.BaseResponse  "Not authorized to represent this organization"
// @Failure      404  {object}  v1.BaseResponse  "Organization not found"
// @Router       /users/me/eid/organizations [post]
func (h Handler) AddOrganization(w http.ResponseWriter, r *http.Request) error {
	const funcName = "AddEIDOrganization"
	ctx := r.Context()

	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}

	var req requests.EIDOrgRegisterRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	reps, err := h.usecase.RegisterEIDOrganization(ctx, user.ID, req.RegNo)
	if err != nil {
		logger.ErrorWithContext(ctx, "AddEIDOrganization failed", logger.Fields{
			"controller": "eidprofile", "method": funcName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid organization linked",
		responses.FromEIDRepresentations(reps))
}

// RemoveOrganization godoc
// @Summary      Байгууллага салгах (eID)
// @Description  Нэвтэрсэн иргэн өөрийн төлөөлдөг байгууллагын (регистрийн дугаараар) холбоосыг цуцлана. Иргэний үлдсэн төлөөлдөг байгууллагыг буцаана.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        regNo  path  string  true  "Байгууллагын регистрийн дугаар"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgRepresentationResponse}
// @Router       /users/me/eid/organizations/{regNo} [delete]
func (h Handler) RemoveOrganization(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	reps, err := h.usecase.UnlinkEIDOrganization(ctx, user.ID, chi.URLParam(r, "regNo"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid organization unlinked",
		responses.FromEIDRepresentations(reps))
}

// OrgSigners godoc
// @Summary      Байгууллагын гарын үсэг зурагчид (eID)
// @Description  Нэвтэрсэн иргэний төлөөлдөг байгууллагын гарын үсэг зурах эрхтэй хүмүүс. Иргэн тухайн байгууллагын төлөөлөгч байх ёстой.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        regNo  path  string  true  "Байгууллагын регистрийн дугаар"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgSignerResponse}
// @Router       /users/me/eid/organizations/{regNo}/signers [get]
func (h Handler) OrgSigners(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	signers, err := h.usecase.ListEIDOrgSigners(ctx, user.ID, chi.URLParam(r, "regNo"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid org signers fetched",
		responses.FromEIDSigners(signers))
}

// AddOrgSigner godoc
// @Summary      Гарын үсэг зурагч нэмэх (eID)
// @Description  Нэвтэрсэн иргэний төлөөлдөг байгууллагад өөр eID иргэнийг (регистрийн дугаараар) гарын үсэг зурах эрхтэй болгож нэмнэ.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        regNo    path  string                          true  "Байгууллагын регистрийн дугаар"
// @Param        payload  body  requests.AddEIDSignerRequest    true  "Гарын үсэг зурагчийн РД + үүрэг"
// @Success      200  {object}  v1.BaseResponse{data=responses.OrgSignersResultResponse}  "Signers + pending confirmation"
// @Router       /users/me/eid/organizations/{regNo}/signers [post]
func (h Handler) AddOrgSigner(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	var req requests.AddEIDSignerRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	res, err := h.usecase.AddEIDOrgSigner(ctx, user.ID, chi.URLParam(r, "regNo"), req.SignerRegNo, req.Role)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid org signer added",
		responses.FromEIDSignersResult(res))
}

// ResendOrgSigner godoc
// @Summary      Баталгаажуулах хүсэлт дахин илгээх (eID)
// @Description  Баталгаажаагүй (PENDING) гарын үсэг зурагч руу eID sign-push баталгаажуулах хүсэлтийг дахин илгээнэ.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        regNo   path   string  true  "Байгууллагын регистрийн дугаар"
// @Param        signer  query  string  true  "Гарын үсэг зурагчийн РД"
// @Success      200  {object}  v1.BaseResponse{data=responses.OrgSignersResultResponse}
// @Router       /users/me/eid/organizations/{regNo}/signers/resend [post]
func (h Handler) ResendOrgSigner(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	res, err := h.usecase.ResendEIDOrgSigner(ctx, user.ID, chi.URLParam(r, "regNo"), r.URL.Query().Get("signer"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid org signer confirmation resent",
		responses.FromEIDSignersResult(res))
}

// RemoveOrgSigner godoc
// @Summary      Гарын үсэг зурагч хасах (eID)
// @Description  Нэвтэрсэн иргэний төлөөлдөг байгууллагаас гарын үсэг зурагчийг (регистрийн дугаараар) хасна.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        regNo   path   string  true  "Байгууллагын регистрийн дугаар"
// @Param        signer  query  string  true  "Хасах гарын үсэг зурагчийн РД"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.OrgSignerResponse}
// @Router       /users/me/eid/organizations/{regNo}/signers [delete]
func (h Handler) RemoveOrgSigner(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	signers, err := h.usecase.RemoveEIDOrgSigner(ctx, user.ID, chi.URLParam(r, "regNo"), r.URL.Query().Get("signer"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid org signer removed",
		responses.FromEIDSigners(signers))
}

// Summary godoc
// @Summary      eID PKI самбарын нэгдсэн тоо
// @Description  Нэвтэрсэн иргэний гэрчилгээ/auth-sign/төхөөрөмж/байгууллагын нэгдсэн тоолол (PKI_READ эрхтэй RP). Эрхгүй бол 403.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=responses.EIDSummaryResponse}
// @Router       /users/me/eid/summary [get]
func (h Handler) Summary(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	res, err := h.usecase.EIDSummary(r.Context(), user.ID)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid summary fetched", responses.FromEIDSummary(res))
}

// Certificates godoc
// @Summary      eID гэрчилгээний жагсаалт + тоо
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=responses.EIDCertificatesResponse}
// @Router       /users/me/eid/certificates [get]
func (h Handler) Certificates(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	res, err := h.usecase.EIDCertificates(r.Context(), user.ID)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid certificates fetched", responses.FromEIDCertificates(res))
}

// Devices godoc
// @Summary      eID холбоотой төхөөрөмжүүд
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=responses.EIDDevicesResponse}
// @Router       /users/me/eid/devices [get]
func (h Handler) Devices(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	res, err := h.usecase.EIDDevices(r.Context(), user.ID)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid devices fetched", responses.FromEIDDevices(res))
}

// Activity godoc
// @Summary      eID auth/sign түүх + тоо (RP-scoped)
// @Tags         users
// @Produce      json
// @Param        limit   query  int  false  "хуудасны хэмжээ (default 20)"
// @Param        offset  query  int  false  "эхлэх байрлал"
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=responses.EIDActivityResponse}
// @Router       /users/me/eid/activity [get]
func (h Handler) Activity(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	res, err := h.usecase.EIDActivity(r.Context(), user.ID, limit, offset)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid activity fetched", responses.FromEIDActivity(res))
}
