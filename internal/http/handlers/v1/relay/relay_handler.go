// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package relay нь platform-хоорондын хүсэлт дамжуулах + SLA хяналтын endpoint-
// уудыг үйлчилнэ. Ingest/Respond нь m2m OAuth (svc:relay); dashboard/CRUD нь
// admin (relay.view / relay.manage).
package relay

import (
	"io"
	"net/http"
	"strconv"

	"template/internal/business/domain"
	relayuc "template/internal/business/usecases/relay"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	usecase relayuc.Usecase
}

func NewHandler(usecase relayuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

func decode[T any](w http.ResponseWriter, r *http.Request, req *T) bool {
	if err := v1.DecodeBody(r, req); err != nil {
		_ = v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := validators.ValidatePayloads(*req); err != nil {
		_ = v1.RespondWithError(w, r, err)
		return false
	}
	return true
}

// ── Ingest / Respond (m2m) ───────────────────────────────────────────────────

// Ingest godoc
// @Summary      Дээд platform-оос хугацаатай үйлчилгээний хүсэлт хүлээж авах
// @Description  service_code-ийн routing дүрмээр доод platform-ууд руу дамжуулж, SLA хяналтад авна.
// @Tags         relay
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  requests.RelayIngestRequest  true  "Хүсэлт"
// @Success      201  {object}  v1.BaseResponse{data=responses.RelayRequestResponse}
// @Router       /relay/requests [post]
func (h Handler) Ingest(w http.ResponseWriter, r *http.Request) error {
	var req requests.RelayIngestRequest
	if !decode(w, r, &req) {
		return nil
	}
	out, err := h.usecase.Ingest(r.Context(), relayuc.IngestInput{
		SourcePlatform: req.SourcePlatform, ExternalRef: req.ExternalRef, ServiceCode: req.ServiceCode,
		Title: req.Title, Payload: req.Payload, Priority: req.Priority, DueAt: req.DueAt,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "request accepted", responses.FromRelayRequest(out))
}

// Respond godoc
// @Summary      Доод platform-ын callback — даалгаврын хариу
// @Tags         relay
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Assignment ID"
// @Param        body  body  requests.RelayRespondRequest  true  "Хариу"
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/assignments/{id}/respond [post]
func (h Handler) Respond(w http.ResponseWriter, r *http.Request) error {
	var req requests.RelayRespondRequest
	if !decode(w, r, &req) {
		return nil
	}
	if err := h.usecase.Respond(r.Context(), chi.URLParam(r, "id"), relayuc.RespondInput{Status: req.Status, Result: req.Result}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "response recorded", nil)
}

// ReceiveWebhook godoc
// @Summary      Peer platform-оос webhook хүлээж авах (дээш/доош, HMAC гарын үсэгтэй)
// @Description  X-Relay-Source (илгээгчийн code) + X-Relay-Signature (sha256=HMAC) header-аар баталгаажуулж, шинэ хүсэлт болгон ingest хийнэ. JWT шаардахгүй.
// @Tags         relay
// @Accept       json
// @Produce      json
// @Param        X-Relay-Source     header  string  true  "Илгээгч platform-ын code"
// @Param        X-Relay-Signature  header  string  true  "sha256=<HMAC-SHA256>"
// @Success      201  {object}  v1.BaseResponse{data=responses.RelayRequestResponse}
// @Router       /relay/webhook [post]
func (h Handler) ReceiveWebhook(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB дээд хязгаар
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "webhook бие уншиж чадсангүй")
	}
	out, err := h.usecase.ReceiveWebhook(r.Context(),
		r.Header.Get(domain.RelayWebhookSourceHeader), r.Header.Get(domain.RelayWebhookSigHeader), body)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "webhook accepted", responses.FromRelayRequest(out))
}

// ForwardUp godoc
// @Summary      Хүсэлтийг дээд (upstream) platform руу webhook-оор дамжуулах
// @Tags         relay
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Request ID"
// @Param        body  body  requests.RelayForwardRequest  true  "Дамжуулах дээд platform"
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/requests/{id}/forward [post]
func (h Handler) ForwardUp(w http.ResponseWriter, r *http.Request) error {
	var req requests.RelayForwardRequest
	if !decode(w, r, &req) {
		return nil
	}
	if err := h.usecase.ForwardUp(r.Context(), chi.URLParam(r, "id"), req.PlatformCode); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "forwarded upstream", nil)
}

// ── Dashboard (admin) ────────────────────────────────────────────────────────

// Overview godoc
// @Summary      SLA хяналтын самбарын нэгтгэл
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=responses.RelayOverviewResponse}
// @Router       /relay/overview [get]
func (h Handler) Overview(w http.ResponseWriter, r *http.Request) error {
	o, err := h.usecase.Overview(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "overview fetched", responses.FromRelayOverview(o))
}

// ListRequests godoc
// @Summary      Хүсэлтүүдийн жагсаалт
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Param        limit  query  int  false  "Max rows (default 50, max 200)"
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/requests [get]
func (h Handler) ListRequests(w http.ResponseWriter, r *http.Request) error {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.usecase.ListRequests(r.Context(), limit)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "requests fetched", responses.ToRelayRequestList(list))
}

// GetRequest godoc
// @Summary      Хүсэлтийн дэлгэрэнгүй (assignments + timeline)
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Request ID"
// @Success      200  {object}  v1.BaseResponse{data=responses.RelayRequestDetailResponse}
// @Router       /relay/requests/{id} [get]
func (h Handler) GetRequest(w http.ResponseWriter, r *http.Request) error {
	d, err := h.usecase.GetRequest(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "request fetched", responses.FromRelayRequestDetail(d))
}

// ── Platforms / routes (admin, relay.manage) ─────────────────────────────────

// ListPlatforms godoc
// @Summary      Доод platform-уудыг жагсаах
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/platforms [get]
func (h Handler) ListPlatforms(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListPlatforms(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "platforms fetched", responses.ToRelayPlatformList(list))
}

// CreatePlatform godoc
// @Summary      Доод platform бүртгэх
// @Tags         relay
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  requests.RelayPlatformRequest  true  "Platform"
// @Success      201  {object}  v1.BaseResponse
// @Router       /relay/platforms [post]
func (h Handler) CreatePlatform(w http.ResponseWriter, r *http.Request) error {
	var req requests.RelayPlatformRequest
	if !decode(w, r, &req) {
		return nil
	}
	p, err := h.usecase.CreatePlatform(r.Context(), relayuc.PlatformInput{
		Code: req.Code, Name: req.Name, Direction: req.Direction, EndpointURL: req.EndpointURL,
		SupervisorContact: req.SupervisorContact, WebhookSecret: req.WebhookSecret, Enabled: req.Enabled,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "platform created", responses.FromRelayPlatform(p))
}

// DeletePlatform godoc
// @Summary      Доод platform устгах
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Platform ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/platforms/{id} [delete]
func (h Handler) DeletePlatform(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeletePlatform(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "platform deleted", nil)
}

// ListRoutes godoc
// @Summary      Чиглүүлэлтийн дүрмүүдийг жагсаах
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/routes [get]
func (h Handler) ListRoutes(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListRoutes(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "routes fetched", responses.ToRelayRouteList(list))
}

// CreateRoute godoc
// @Summary      Чиглүүлэлт үүсгэх (service_code → platform)
// @Tags         relay
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  requests.RelayRouteRequest  true  "Route"
// @Success      201  {object}  v1.BaseResponse
// @Router       /relay/routes [post]
func (h Handler) CreateRoute(w http.ResponseWriter, r *http.Request) error {
	var req requests.RelayRouteRequest
	if !decode(w, r, &req) {
		return nil
	}
	rt, err := h.usecase.CreateRoute(r.Context(), relayuc.RouteInput{ServiceCode: req.ServiceCode, PlatformID: req.PlatformID, SLAMinutes: req.SLAMinutes})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "route created", responses.FromRelayRoute(rt))
}

// DeleteRoute godoc
// @Summary      Чиглүүлэлт устгах
// @Tags         relay
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Route ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /relay/routes/{id} [delete]
func (h Handler) DeleteRoute(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeleteRoute(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "route deleted", nil)
}
