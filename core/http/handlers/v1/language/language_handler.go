// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package language нь интерфейсийн хэлийг үйлчилнэ — нэвтрээгүй зочны уншдаг
// идэвхтэй хэлний жагсаалт ба dictionary, мөн super admin-ий удирдлага
// (нэмэх/хасах/идэвхжүүлэх, орчуулга бөглөх).
package language

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	languageuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/language"
	"github.com/gerege-systems/open-gerege-core/core/http/datatransfers/requests"
	"github.com/gerege-systems/open-gerege-core/core/http/datatransfers/responses"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/open-gerege-core/pkg/validators"
)

// dictionaryBodyMaxBytes нь dictionary агуулсан body-ийн дээд хэмжээ. Анхдагч
// 1 MiB нь бүтэн dictionary-д (мянга орчим түлхүүр) хүрэлцэхгүй байж болзошгүй
// бөгөөд таслагдсан body нь "invalid JSON" гэсэн төөрөгдүүлэх алдаа өгнө.
// Энэ хязгаарыг domain.LanguageDictionaryMaxBytes-аас дээгүүр тавьсан нь
// ойлгомжтой алдааг (хэт том dictionary) domain давхаргаас гаргуулах зорилготой.
const dictionaryBodyMaxBytes = 4 << 20

type Handler struct {
	usecase languageuc.Usecase
}

func NewHandler(usecase languageuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// ListEnabled godoc
// @Summary Идэвхтэй хэлүүдийг унших
// @Description Хэрэглэгчид харагдах хэлний жагсаалт. Нэвтрэлт шаардахгүй.
// @Tags site
// @Produce json
// @Success 200 {object} v1.BaseResponse{data=[]responses.LanguageResponse}
// @Router /languages/enabled [get]
func (h Handler) ListEnabled(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListEnabled(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "enabled languages fetched", responses.ToLanguageList(list))
}

// Dictionary godoc
// @Summary Хэлний орчуулгын dictionary унших
// @Description Тухайн хэлний түлхүүр→утга газрын зураг. Хоосон байвал апп өөрийн багцлагдсан утгаа хэрэглэнэ. Нэвтрэлт шаардахгүй.
// @Tags site
// @Produce json
// @Param code path string true "Хэлний код"
// @Success 200 {object} v1.BaseResponse{data=responses.LanguageDictionaryResponse}
// @Router /languages/{code}/dictionary [get]
func (h Handler) Dictionary(w http.ResponseWriter, r *http.Request) error {
	code := chi.URLParam(r, "code")
	entries, err := h.usecase.Dictionary(r.Context(), code)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "dictionary fetched", responses.ToLanguageDictionary(code, entries))
}

// List godoc
// @Summary Бүх хэлийг жагсаах
// @Tags superadmin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} v1.BaseResponse{data=[]responses.LanguageResponse}
// @Router /languages [get]
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.List(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "languages fetched", responses.ToLanguageList(list))
}

// Create godoc
// @Summary Шинэ хэл нэмэх
// @Description Хэл нь унтраалттай үүснэ — орчуулгыг бөглөсний дараа идэвхжүүлнэ.
// @Tags superadmin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body requests.LanguageCreateRequest true "Language"
// @Success 200 {object} v1.BaseResponse{data=responses.LanguageResponse}
// @Router /languages [post]
func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
	var req requests.LanguageCreateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	lang, err := h.usecase.Create(r.Context(), req.Code, req.Label, req.Locale)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "language created", responses.ToLanguage(lang))
}

// Update godoc
// @Summary Хэлийг шинэчлэх (идэвхжүүлэх/унтраах, нэр, эрэмбэ)
// @Tags superadmin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param code path string true "Хэлний код"
// @Param request body requests.LanguageUpdateRequest true "Patch"
// @Success 200 {object} v1.BaseResponse
// @Router /languages/{code} [patch]
func (h Handler) Update(w http.ResponseWriter, r *http.Request) error {
	var req requests.LanguageUpdateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	patch := domainPatch(req)
	if err := h.usecase.Update(r.Context(), chi.URLParam(r, "code"), patch); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "language updated", nil)
}

// Delete godoc
// @Summary Хэлийг устгах
// @Description Багцлагдсан (built-in) хэлийг устгаж болохгүй — зөвхөн унтраана.
// @Tags superadmin
// @Produce json
// @Security BearerAuth
// @Param code path string true "Хэлний код"
// @Success 200 {object} v1.BaseResponse
// @Router /languages/{code} [delete]
func (h Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.Delete(r.Context(), chi.URLParam(r, "code")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "language deleted", nil)
}

// PutTranslations godoc
// @Summary Орчуулгыг гараар бичих
// @Description Оруулсан түлхүүрүүдийг 'manual' болгож тэмдэглэнэ — дараагийн автомат орчуулга эдгээрийг дарж бичихгүй.
// @Tags superadmin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param code path string true "Хэлний код"
// @Param request body requests.LanguageTranslationsRequest true "Entries"
// @Success 200 {object} v1.BaseResponse
// @Router /languages/{code}/translations [put]
func (h Handler) PutTranslations(w http.ResponseWriter, r *http.Request) error {
	var req requests.LanguageTranslationsRequest
	if err := v1.DecodeBodyLimit(r, &req, dictionaryBodyMaxBytes); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.PutTranslations(r.Context(), chi.URLParam(r, "code"), req.Entries); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "translations saved", nil)
}

// AutoTranslate godoc
// @Summary Хэлийг AI-аар суулгах (дутуу орчуулгыг Gemini-ээр бөглөх)
// @Description Аппын эх хэлний dictionary-г илгээнэ; дутуу түлхүүрүүд орчуулагдана. Байрлуулагч ({name}) алдагдсан орчуулга суухгүй.
// @Tags superadmin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param code path string true "Хэлний код"
// @Param request body requests.LanguageAutoTranslateRequest true "Base dictionary"
// @Success 200 {object} v1.BaseResponse{data=responses.LanguageAutoTranslateResponse}
// @Router /languages/{code}/translate [post]
func (h Handler) AutoTranslate(w http.ResponseWriter, r *http.Request) error {
	var req requests.LanguageAutoTranslateRequest
	if err := v1.DecodeBodyLimit(r, &req, dictionaryBodyMaxBytes); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	result, err := h.usecase.AutoTranslate(r.Context(), languageuc.AutoTranslateRequest{
		Code:      chi.URLParam(r, "code"),
		Base:      req.Base,
		BaseLang:  req.BaseLang,
		Overwrite: req.Overwrite,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "translation finished", responses.ToLanguageAutoTranslate(result))
}

// domainPatch нь хүсэлтийн заагчуудыг domain-ий хэсэгчилсэн шинэчлэлт болгоно.
// nil нь "хэвээр" гэсэн утгатай тул шууд дамжуулна.
func domainPatch(req requests.LanguageUpdateRequest) domain.LanguagePatch {
	return domain.LanguagePatch{
		Label:     req.Label,
		Locale:    req.Locale,
		Enabled:   req.Enabled,
		SortOrder: req.SortOrder,
	}
}
