// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package sign — PDF гарын үсгийн (PAdES) HTTP handler. Хувь хүн.
package sign

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"template/internal/apperror"
	signuc "template/internal/business/usecases/sign"
	"template/internal/business/usecases/users"
	httpauth "template/internal/http/auth"
	v1 "template/internal/http/handlers/v1"
)

const maxUpload = 26 << 20 // 25 MB + overhead

// Handler — sign + users usecase.
type Handler struct {
	sign  signuc.Usecase
	users users.Usecase
}

func NewHandler(s signuc.Usecase, u users.Usecase) Handler { return Handler{sign: s, users: u} }

// currentRegNo — нэвтэрсэн иргэний регистрийн дугаар. ЭНЭ template-д eID
// хэрэглэгчийн Username нь "eid_"+civil_id (регистр БИШ) тул регистрийг
// domain.User.NationalID талбараас авна. Энэ утга нь sign session-ы
// эзэмшигчийн түлхүүр — Init дээр хадгалагдаж, Poll/Download дээр тулгагдана
// (IDOR-аас хамгаална). Регистр хоосон бол цэвэр BadRequest.
func (h Handler) currentRegNo(r *http.Request) (string, error) {
	cu, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return "", err
	}
	ures, err := h.users.GetByID(r.Context(), users.GetByIDRequest{ID: cu.ID})
	if err != nil {
		return "", err
	}
	regNo := strings.TrimSpace(ures.User.NationalID)
	if regNo == "" {
		return "", apperror.BadRequest("eID регистрийн дугаар олдсонгүй")
	}
	return regNo, nil
}

// Init godoc
// @Summary      PDF гарын үсэг эхлүүлэх (eID PIN2)
// @Description  Нэвтэрсэн иргэний eID регистрээр /v3 PIN2 гарын үсэг эхлүүлж, session_id + verification_code буцаана. Иргэн утсан дээрээ PIN2-оор зөвшөөрнө.
// @Tags         sign
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "Гарын үсэг зурах PDF (≤25MB)"
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse  "session_id + verification_code"
// @Failure      400  {object}  v1.BaseResponse  "invalid form / регистр олдсонгүй"
// @Failure      401  {object}  v1.BaseResponse  "unauthorized"
// @Router       /v1/sign/init [post]
func (h Handler) Init(w http.ResponseWriter, r *http.Request) error {
	cu, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, "unauthorized")
	}
	ures, err := h.users.GetByID(r.Context(), users.GetByIDRequest{ID: cu.ID})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	u := ures.User
	// ЭНЭ template-д eID хэрэглэгчийн Username нь "eid_"+civil_id тул регистрийг
	// NationalID-аас авна. Public-RP хэрэглэгчид РД байхгүй байж болзошгүй —
	// panic биш, цэвэр BadRequest.
	regNo := strings.TrimSpace(u.NationalID)
	if regNo == "" {
		return v1.RespondWithError(w, r, apperror.BadRequest("eID регистрийн дугаар олдсонгүй"))
	}
	// #nosec G120 — maxUpload (26 MiB) нь тодорхой дээд хязгаар; memory exhaustion хаалттай.
	err = r.ParseMultipartForm(maxUpload)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid form")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "file required")
	}
	defer func() { _ = f.Close() }()
	body, err := io.ReadAll(io.LimitReader(f, maxUpload))
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "read failed")
	}
	name := strings.TrimSpace(u.LastName + " " + u.FirstName)
	res, err := h.sign.Init(r.Context(), regNo, name, hdr.Filename, body)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", res)
}

// Poll godoc
// @Summary      Гарын үсгийн session төлөв
// @Description  Session-ийн төлөв (running|completed|failed|rejected). Зөвхөн эзэмшигч иргэн хандана.
// @Tags         sign
// @Produce      json
// @Param        id  path  string  true  "session_id"
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=map[string]string}
// @Failure      401  {object}  v1.BaseResponse  "unauthorized"
// @Failure      404  {object}  v1.BaseResponse  "session олдсонгүй"
// @Router       /v1/sign/{id} [get]
func (h Handler) Poll(w http.ResponseWriter, r *http.Request) error {
	regNo, err := h.currentRegNo(r)
	if err != nil {
		if _, ok := err.(*apperror.DomainError); ok {
			return v1.RespondWithError(w, r, err)
		}
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, "unauthorized")
	}
	state, err := h.sign.Poll(r.Context(), regNo, chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "ok", map[string]string{"state": state})
}

// Download godoc
// @Summary      Гарын үсэгтэй PDF татах
// @Description  PAdES гарын үсэг шигтгэсэн PDF-ийг урсгана. Зөвхөн эзэмшигч иргэн, completed session.
// @Tags         sign
// @Produce      application/pdf
// @Param        id  path  string  true  "session_id"
// @Security     BearerAuth
// @Success      200  {file}  binary
// @Failure      401  {object}  v1.BaseResponse  "unauthorized"
// @Failure      404  {object}  v1.BaseResponse  "session олдсонгүй"
// @Router       /v1/sign/{id}/download [get]
func (h Handler) Download(w http.ResponseWriter, r *http.Request) error {
	regNo, err := h.currentRegNo(r)
	if err != nil {
		if _, ok := err.(*apperror.DomainError); ok {
			return v1.RespondWithError(w, r, err)
		}
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, "unauthorized")
	}
	res, err := h.sign.Download(r.Context(), regNo, chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+res.Filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res.PDF)
	return nil
}
