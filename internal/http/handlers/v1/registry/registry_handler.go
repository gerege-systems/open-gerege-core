// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package registry нь Ring System · R1 — Үйлчилгээний нэгдсэн регистрийн
// /registry/* endpoint-уудыг үйлчилнэ. Уншилт нь 'registry.view', бичилт нь
// 'registry.manage' эрх шаардана (route_registry.go); нийтийн каталог нь
// зөвхөн нийтлэгдсэн паспортыг харуулна.
package registry

import (
	"net/http"

	registryuc "template/internal/business/usecases/registry"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	usecase registryuc.Usecase
}

func NewHandler(usecase registryuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// pathID нь URL-ийн {id}-г задалж хүчинтэй UUID эсэхийг шалгана. Буруу бол 400
// бичээд false буцаана (дуудагч nil буцаана — Wrap дахин бичихгүй). Ингэснээр
// UUID биш id нь Postgres-ийн "invalid input syntax" 500-ийн оронд цэвэр 400 болно.
func pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		_ = v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid id")
		return "", false
	}
	return id, true
}

// filterFromQuery нь жагсаалтын шүүлтүүрийг query параметрээс уншина.
func filterFromQuery(r *http.Request) registryuc.ListFilter {
	q := r.URL.Query()
	return registryuc.ListFilter{
		Status:      q.Get("status"),
		Authority:   q.Get("authority"),
		LifeEventID: q.Get("life_event_id"),
		Proactivity: q.Get("proactivity"),
		Query:       q.Get("q"),
	}
}

// actorID нь нийтлэлтийг хэн хийснийг тэмдэглэхэд ашиглагдана (аудитын мөр).
// Токенгүй/буруу бол nil — нийтлэлт өөрөө зогсохгүй (эрхийг middleware шалгасан).
func actorID(r *http.Request) *string {
	u, err := httpauth.CurrentUserFromContext(r)
	if err != nil || u.ID == "" {
		return nil
	}
	id := u.ID
	return &id
}

// toServiceInput нь паспортын body-г usecase оролт руу хөрвүүлнэ.
func toServiceInput(req requests.RegistryServiceRequest) registryuc.ServiceInput {
	return registryuc.ServiceInput{
		Code: req.Code, Name: req.Name, NameEN: req.NameEN, Description: req.Description,
		Authority: req.Authority, AuthorityOrgID: req.AuthorityOrgID, LegalBasis: req.LegalBasis,
		TargetGroup: req.TargetGroup, Output: req.Output, Channels: req.Channels,
		Fee: req.Fee, MaxDays: req.MaxDays, StepsCount: req.StepsCount,
		AnnualVolume: req.AnnualVolume, Proactivity: req.Proactivity, LifeEventID: req.LifeEventID,
	}
}

// ── Нэгтгэл ба нийтийн каталог ────────────────────────────────────────────—

// Overview godoc
// @Summary      Регистрийн нэгтгэл (инвентар, once-only, проактив байдал)
// @Tags         registry
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/overview [get]
func (h Handler) Overview(w http.ResponseWriter, r *http.Request) error {
	o, err := h.usecase.Overview(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "registry overview fetched successfully", responses.FromRegistryOverview(o))
}

// Catalog godoc
// @Summary      Нийтийн каталог (зөвхөн нийтлэгдсэн паспорт)
// @Tags         registry
// @Produce      json
// @Param        q             query  string  false  "Нэр/код дотор хайх"
// @Param        authority     query  string  false  "Эрх бүхий байгууллага"
// @Param        life_event_id query  string  false  "Амьдралын үйл явдал"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/catalog [get]
func (h Handler) Catalog(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.PublicCatalog(r.Context(), filterFromQuery(r))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "catalog fetched successfully", responses.ToRegistryServiceList(list))
}

// PublicService godoc
// @Summary      Нийтийн каталогийн нэг үйлчилгээ (зөвхөн нийтлэгдсэн)
// @Tags         catalog
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /catalog/services/{id} [get]
func (h Handler) PublicService(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	s, err := h.usecase.PublicService(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service fetched successfully", responses.FromRegistryService(s))
}

// PublicLifeEvents godoc
// @Summary      Нийтийн каталогийн амьдралын үйл явдлууд
// @Tags         catalog
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /catalog/life-events [get]
func (h Handler) PublicLifeEvents(w http.ResponseWriter, r *http.Request) error {
	return h.ListLifeEvents(w, r)
}

// ── Паспорт ───────────────────────────────────────────────────────────────—

// ListServices godoc
// @Summary      Үйлчилгээний паспортын жагсаалт (ноорог орно)
// @Tags         registry
// @Produce      json
// @Param        status      query  string  false  "draft/published/archived"
// @Param        authority   query  string  false  "Эрх бүхий байгууллага"
// @Param        proactivity query  string  false  "Проактив байдлын шат"
// @Param        q           query  string  false  "Нэр/код дотор хайх"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services [get]
func (h Handler) ListServices(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListServices(r.Context(), filterFromQuery(r))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "services fetched successfully", responses.ToRegistryServiceList(list))
}

// GetService godoc
// @Summary      Үйлчилгээний паспорт (нотолгоотой нь)
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id} [get]
func (h Handler) GetService(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	s, err := h.usecase.GetService(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service fetched successfully", responses.FromRegistryService(s))
}

// CreateService godoc
// @Summary      Үйлчилгээний паспорт үүсгэх (ноорог)
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        body  body  requests.RegistryServiceRequest  true  "Service"
// @Success      201  {object}  v1.BaseResponse
// @Router       /registry/services [post]
func (h Handler) CreateService(w http.ResponseWriter, r *http.Request) error {
	var req requests.RegistryServiceRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	s, err := h.usecase.CreateService(r.Context(), toServiceInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "service created successfully", responses.FromRegistryService(s))
}

// UpdateService godoc
// @Summary      Үйлчилгээний паспорт засах
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        id    path  string                           true  "Service ID"
// @Param        body  body  requests.RegistryServiceRequest  true  "Service"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id} [put]
func (h Handler) UpdateService(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	var req requests.RegistryServiceRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	s, err := h.usecase.UpdateService(r.Context(), id, toServiceInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service updated successfully", responses.FromRegistryService(s))
}

// DeleteService godoc
// @Summary      Ноорог паспорт устгах (нийтлэгдсэнийг устгахгүй — архивлана)
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id} [delete]
func (h Handler) DeleteService(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	if err := h.usecase.DeleteService(r.Context(), id); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service deleted successfully", nil)
}

// ArchiveService godoc
// @Summary      Паспортыг архивлах
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id}/archive [post]
func (h Handler) ArchiveService(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	if err := h.usecase.ArchiveService(r.Context(), id); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service archived successfully", nil)
}

// ── Нотолгооны холбоос, нийтлэлт, хувилбар ────────────────────────────────—

// SetEvidences godoc
// @Summary      Паспортын шаардах нотолгооны жагсаалтыг солих
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        id    path  string                             true  "Service ID"
// @Param        body  body  requests.RegistryEvidencesRequest  true  "Evidences"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id}/evidences [put]
func (h Handler) SetEvidences(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	var req requests.RegistryEvidencesRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	links := make([]registryuc.EvidenceLink, 0, len(req.Evidences))
	for _, e := range req.Evidences {
		links = append(links, registryuc.EvidenceLink{
			EvidenceID: e.EvidenceID, Required: e.Required, FromCitizen: e.FromCitizen, Note: e.Note,
		})
	}
	s, err := h.usecase.SetEvidences(r.Context(), id, links)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "evidences updated successfully", responses.FromRegistryService(s))
}

// Publish godoc
// @Summary      Паспортыг нийтлэх (шинэ хувилбар + baseline delta)
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        id    path  string                           true   "Service ID"
// @Param        body  body  requests.RegistryPublishRequest  false  "Publish"
// @Success      201  {object}  v1.BaseResponse
// @Router       /registry/services/{id}/publish [post]
func (h Handler) Publish(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	// Body сонголттой — хоосон бол тэмдэглэлгүй нийтэлнэ.
	var req requests.RegistryPublishRequest
	if r.ContentLength > 0 {
		if err := v1.DecodeBody(r, &req); err != nil {
			return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
		}
		if err := validators.ValidatePayloads(req); err != nil {
			return v1.RespondWithError(w, r, err)
		}
	}
	v, err := h.usecase.Publish(r.Context(), id, registryuc.PublishInput{
		ChangeNote: req.ChangeNote, PublishedBy: actorID(r),
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "service published successfully", responses.FromRegistryVersion(v))
}

// ListVersions godoc
// @Summary      Паспортын хувилбарын түүх (baseline delta-тай)
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id}/versions [get]
func (h Handler) ListVersions(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	list, err := h.usecase.ListVersions(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "versions fetched successfully", responses.ToRegistryVersionList(list))
}

// ── Once-only ─────────────────────────────────────────────────────────────—

// CheckOnceOnly godoc
// @Summary      Нэг үйлчилгээний once-only шалгалт
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/services/{id}/once-only [get]
func (h Handler) CheckOnceOnly(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	rep, err := h.usecase.CheckOnceOnly(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "once-only check completed", responses.FromRegistryOnceOnlyReport(rep))
}

// OnceOnlyViolations godoc
// @Summary      Once-only зөрчлүүд (ХУР-д байгааг иргэнээс дахин шаардаж буй)
// @Tags         registry
// @Produce      json
// @Param        authority  query  string  false  "Эрх бүхий байгууллагаар шүүх"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/once-only [get]
func (h Handler) OnceOnlyViolations(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.OnceOnlyViolations(r.Context(), r.URL.Query().Get("authority"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "once-only violations fetched successfully", responses.ToRegistryOnceOnlyList(list))
}

// ── Нотолгооны каталог ────────────────────────────────────────────────────—

// ListEvidences godoc
// @Summary      Нотолгооны каталог (ХУР mapping-тай)
// @Tags         registry
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/evidences [get]
func (h Handler) ListEvidences(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListEvidences(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "evidences fetched successfully", responses.ToRegistryEvidenceList(list))
}

// CreateEvidence godoc
// @Summary      Нотолгоо бүртгэх
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        body  body  requests.RegistryEvidenceRequest  true  "Evidence"
// @Success      201  {object}  v1.BaseResponse
// @Router       /registry/evidences [post]
func (h Handler) CreateEvidence(w http.ResponseWriter, r *http.Request) error {
	var req requests.RegistryEvidenceRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	e, err := h.usecase.CreateEvidence(r.Context(), registryuc.EvidenceInput{
		Code: req.Code, Name: req.Name, Description: req.Description,
		HolderAgency: req.HolderAgency, SourceSystem: req.SourceSystem,
		InKHUR: req.InKHUR, KHURServiceCode: req.KHURServiceCode,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "evidence created successfully", responses.FromRegistryEvidence(e))
}

// UpdateEvidence godoc
// @Summary      Нотолгоо засах (ХУР боломж тэмдэглэх)
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        id    path  string                            true  "Evidence ID"
// @Param        body  body  requests.RegistryEvidenceRequest  true  "Evidence"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/evidences/{id} [put]
func (h Handler) UpdateEvidence(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	var req requests.RegistryEvidenceRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	e, err := h.usecase.UpdateEvidence(r.Context(), id, registryuc.EvidenceInput{
		Name: req.Name, Description: req.Description, HolderAgency: req.HolderAgency,
		SourceSystem: req.SourceSystem, InKHUR: req.InKHUR, KHURServiceCode: req.KHURServiceCode,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "evidence updated successfully", responses.FromRegistryEvidence(e))
}

// DeleteEvidence godoc
// @Summary      Нотолгоо устгах
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Evidence ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/evidences/{id} [delete]
func (h Handler) DeleteEvidence(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	if err := h.usecase.DeleteEvidence(r.Context(), id); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "evidence deleted successfully", nil)
}

// ── Амьдралын үйл явдал ───────────────────────────────────────────────────—

// ListLifeEvents godoc
// @Summary      Амьдралын/бизнесийн үйл явдлууд
// @Tags         registry
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/life-events [get]
func (h Handler) ListLifeEvents(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListLifeEvents(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "life events fetched successfully", responses.ToRegistryLifeEventList(list))
}

// CreateLifeEvent godoc
// @Summary      Амьдралын/бизнесийн үйл явдал үүсгэх
// @Tags         registry
// @Accept       json
// @Produce      json
// @Param        body  body  requests.RegistryLifeEventRequest  true  "Life event"
// @Success      201  {object}  v1.BaseResponse
// @Router       /registry/life-events [post]
func (h Handler) CreateLifeEvent(w http.ResponseWriter, r *http.Request) error {
	var req requests.RegistryLifeEventRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	l, err := h.usecase.CreateLifeEvent(r.Context(), registryuc.LifeEventInput{
		Code: req.Code, Name: req.Name, Kind: req.Kind, Description: req.Description,
		LeadAgency: req.LeadAgency, SortOrder: req.SortOrder,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "life event created successfully", responses.FromRegistryLifeEvent(l))
}

// DeleteLifeEvent godoc
// @Summary      Амьдралын үйл явдал устгах
// @Tags         registry
// @Produce      json
// @Param        id  path  string  true  "Life event ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /registry/life-events/{id} [delete]
func (h Handler) DeleteLifeEvent(w http.ResponseWriter, r *http.Request) error {
	id, ok := pathID(w, r)
	if !ok {
		return nil
	}
	if err := h.usecase.DeleteLifeEvent(r.Context(), id); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "life event deleted successfully", nil)
}
