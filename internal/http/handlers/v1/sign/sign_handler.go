// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package sign — PDF гарын үсгийн (PAdES) HTTP handler. Хувь хүн.
package sign

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"template/internal/apperror"
	assetsuc "template/internal/business/usecases/assets"
	signuc "template/internal/business/usecases/sign"
	"template/internal/business/usecases/users"
	httpauth "template/internal/http/auth"
	v1 "template/internal/http/handlers/v1"
)

const maxUpload = 26 << 20 // 25 MB + overhead

// Handler — sign + users + assets usecase (гарын үсэг/тамганы зургийг PDF-д давхарлана).
type Handler struct {
	sign   signuc.Usecase
	users  users.Usecase
	assets assetsuc.Usecase
}

func NewHandler(s signuc.Usecase, u users.Usecase, a assetsuc.Usecase) Handler {
	return Handler{sign: s, users: u, assets: a}
}

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
// @Param        file        formData  file    true   "Гарын үсэг зурах PDF (≤25MB)"
// @Param        onBehalfOf  formData  string  false  "Байгууллагын etsi (NTRMN-<РД>) — тухайн байгууллагын нэрийн өмнөөс зурах. Хоосон бол хувь хүний гарын үсэг."
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
	// onBehalfOf (NTRMN-<РД>) — сонголтот: тухайн иргэний төлөөлдөг байгууллагын
	// нэрийн өмнөөс зурна. Хоосон бол хувь хүний гарын үсэг. Төлөөллийн эрхийг
	// eidmongolia session үүсгэх үедээ шалгана (эрхгүй бол 403 → Forbidden).
	onBehalfOf := strings.TrimSpace(r.FormValue("onBehalfOf"))
	// Визуал гарын үсэг (хувь хүн) + тамга (байгууллагын нэрийн өмнөөс) зургийн URL —
	// эх PDF-д давхарлахаар sign usecase руу дамжуулна. Best-effort (алдаа → хоосон).
	signatureURL, _ := h.assets.GetSignature(r.Context(), cu.ID)
	stampURL := ""
	if onBehalfOf != "" {
		orgReg := strings.TrimPrefix(strings.ToUpper(onBehalfOf), "NTRMN-")
		stampURL, _ = h.assets.GetStamp(r.Context(), cu.ID, orgReg)
	}
	res, err := h.sign.Init(r.Context(), regNo, name, hdr.Filename, body, onBehalfOf, signatureURL, stampURL)
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
	w.Header().Set("Content-Disposition", contentDisposition(res.Filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res.PDF)
	return nil
}

// contentDisposition нь татах файлын нэрийг зөв дамжуулна. HTTP header-ийн утга
// нь ISO-8859-1 (latin-1) тул кирилл/UTF-8 нэрийг filename="..."-д шууд тавьбал
// browser буруу тайлж "арзайсан" нэр гаргадаг. RFC 5987/6266 дагуу жинхэнэ
// нэрийг filename*=UTF-8”<percent-encoded>-оор өгч, filename*-ийг ойлгодоггүй
// хуучин client-д зориулж ASCII fallback filename="..."-ийг мөн үлдээнэ.
func contentDisposition(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "signed.pdf"
	}
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", asciiFallback(name), rfc5987Escape(name))
}

// asciiFallback нь ASCII бус, control болон quote тэмдэгтүүдийг '_'-оор солино
// (header-т аюулгүй, filename*-гүй client дээр ядаж уншигдахуйц нэр).
func asciiFallback(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || c >= 0x7f || c == '"' || c == '\\' {
			b.WriteByte('_')
		} else {
			b.WriteByte(c)
		}
	}
	if s := strings.Trim(b.String(), "_"); s == "" {
		return "signed.pdf"
	}
	return b.String()
}

// rfc5987Escape нь мөрийг байт тус бүрээр RFC 5987 ext-value болгон percent-encode
// хийнэ (attr-char-аас бусад бүх байтыг %XX болгоно; UTF-8 олон-байт тэмдэгт
// байтаараа кодлогдоно).
func rfc5987Escape(s string) string {
	const hex = "0123456789ABCDEF"
	const attrChars = "!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || strings.IndexByte(attrChars, c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}
