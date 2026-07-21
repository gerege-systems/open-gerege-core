// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gateway нь /gateway/* endpoint-уудыг үйлчилнэ — API Gateway-ийн
// services/routes/consumers/api keys/policies CRUD болон overview/logs
// телеметр. Бүгд 'gateway.manage' эрх шаардана (route давхаргад баталгаажна).
package gateway

import (
	"net/http"
	"strconv"

	gatewayuc "template/internal/business/usecases/gateway"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	usecase gatewayuc.Usecase
}

func NewHandler(usecase gatewayuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// decode нь body-г задлаж, validate хийнэ. Амжилтгүй бол хариуг бичээд false
// буцаана (дуудагч шууд буцна).
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

// ── Overview / Logs ──────────────────────────────────────────────────────—

// Overview godoc
// @Summary      API Gateway-ийн нэгтгэсэн статистик
// @Description  Сүүлийн 24 цагийн хүсэлт/алдааны хувь/латентаас бүрдсэн dashboard нэгтгэл.
// @Tags         gateway
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/overview [get]
func (h Handler) Overview(w http.ResponseWriter, r *http.Request) error {
	o, err := h.usecase.Overview(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "overview fetched successfully", responses.FromGatewayOverview(o))
}

// ListLogs godoc
// @Summary      Gateway-ийн сүүлийн хүсэлтийн log
// @Tags         gateway
// @Produce      json
// @Param        limit  query  int  false  "Max rows (default 100, max 200)"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/logs [get]
func (h Handler) ListLogs(w http.ResponseWriter, r *http.Request) error {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := h.usecase.ListRequestLogs(r.Context(), limit)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "logs fetched successfully", responses.ToGatewayLogList(logs))
}

// ── Services ────────────────────────────────────────────────────────────────

// ListServices godoc
// @Summary      Upstream service-үүдийг жагсаах
// @Tags         gateway
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/services [get]
func (h Handler) ListServices(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListServices(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "services fetched successfully", responses.ToGatewayServiceList(list))
}

// CreateService godoc
// @Summary      Upstream service үүсгэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GatewayServiceRequest  true  "Service"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gateway/services [post]
func (h Handler) CreateService(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayServiceRequest
	if !decode(w, r, &req) {
		return nil
	}
	svc, err := h.usecase.CreateService(r.Context(), svcInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "service created successfully", responses.FromGatewayService(svc))
}

// UpdateService godoc
// @Summary      Upstream service шинэчлэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Service ID"
// @Param        body  body  requests.GatewayServiceRequest  true  "Service"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/services/{id} [put]
func (h Handler) UpdateService(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayServiceRequest
	if !decode(w, r, &req) {
		return nil
	}
	svc, err := h.usecase.UpdateService(r.Context(), chi.URLParam(r, "id"), svcInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service updated successfully", responses.FromGatewayService(svc))
}

// DeleteService godoc
// @Summary      Upstream service устгах
// @Tags         gateway
// @Produce      json
// @Param        id  path  string  true  "Service ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/services/{id} [delete]
func (h Handler) DeleteService(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeleteService(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "service deleted successfully", nil)
}

func svcInput(req requests.GatewayServiceRequest) gatewayuc.ServiceInput {
	return gatewayuc.ServiceInput{
		Name: req.Name, Protocol: req.Protocol, Host: req.Host, Port: req.Port, Path: req.Path,
		Retries: req.Retries, ConnectTimeout: req.ConnectTimeout, Tags: req.Tags, Enabled: req.Enabled,
	}
}
