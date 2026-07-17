// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package landing

import (
	"encoding/json"
	"net/http"

	landinguc "template/internal/business/usecases/landing"
	v1 "template/internal/http/handlers/v1"
)

// Handler нь нүүр хуудасны тохиргооны (landing_config) HTTP гадаргуу —
// нийтийн уншилт + админ бичилт.
type Handler struct {
	usecase landinguc.Usecase
}

func NewHandler(uc landinguc.Usecase) Handler {
	return Handler{usecase: uc}
}

// GetConfig godoc
// @Summary      Нүүр хуудасны тохиргоог авах (нийтийн)
// @Description  Нүүр хуудасны (landing + auth бүрхүүл) харагдацын тохиргоог (өнгө, фонт, хэмжээ, текст mn/en, товч/цэс) буцаана. Нэвтрэлт шаардахгүй — зочид нүүрээ энэ тохиргоогоор буулгана. DB алдаа үед хоосон объект руу fail-open хийнэ.
// @Tags         public
// @Produce      json
// @Success      200  {object}  v1.BaseResponse  "Landing config document"
// @Router       /landing/config [get]
func (h Handler) GetConfig(w http.ResponseWriter, r *http.Request) error {
	cfg := h.usecase.GetConfig(r.Context())
	return v1.NewSuccessResponse(w, r, http.StatusOK, "landing config", cfg)
}

// SetConfig godoc
// @Summary      Нүүр хуудасны тохиргоог шинэчлэх
// @Description  Нүүрний харагдацын тохиргооны баримтыг бүхэлд нь солино. Body нь бүрэн JSON объект (схемийг frontend эзэмшдэг). Хүчинтэй JSON объект + хэмжээний хязгаар (64 KiB) шалгагдаж, advanced CSS override ариутгагдана. Өөрчлөлт нэн даруй үйлчилнэ (тохиргооны кэш хүчингүй болно).
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object  true  "Landing config document"
// @Success      200      {object}  v1.BaseResponse  "Updated"
// @Failure      400      {object}  v1.BaseResponse  "Not a JSON object / too large"
// @Failure      401      {object}  v1.BaseResponse  "Missing/invalid token"
// @Failure      403      {object}  v1.BaseResponse  "Missing settings.manage permission"
// @Router       /admin/landing/config [put]
func (h Handler) SetConfig(w http.ResponseWriter, r *http.Request) error {
	var body json.RawMessage
	if err := v1.DecodeBody(r, &body); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := h.usecase.SetConfig(r.Context(), body); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "landing config updated", nil)
}
