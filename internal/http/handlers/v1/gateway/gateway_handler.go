// Gerege Template Version 27.0
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

// ── Routes ──────────────────────────────────────────────────────────────────

// ListRoutes godoc
// @Summary      Route-уудыг жагсаах
// @Tags         gateway
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/routes [get]
func (h Handler) ListRoutes(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListRoutes(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "routes fetched successfully", responses.ToGatewayRouteList(list))
}

// CreateRoute godoc
// @Summary      Route үүсгэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GatewayRouteRequest  true  "Route"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gateway/routes [post]
func (h Handler) CreateRoute(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayRouteRequest
	if !decode(w, r, &req) {
		return nil
	}
	rt, err := h.usecase.CreateRoute(r.Context(), routeInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "route created successfully", responses.FromGatewayRoute(rt))
}

// UpdateRoute godoc
// @Summary      Route шинэчлэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Route ID"
// @Param        body  body  requests.GatewayRouteRequest  true  "Route"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/routes/{id} [put]
func (h Handler) UpdateRoute(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayRouteRequest
	if !decode(w, r, &req) {
		return nil
	}
	rt, err := h.usecase.UpdateRoute(r.Context(), chi.URLParam(r, "id"), routeInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "route updated successfully", responses.FromGatewayRoute(rt))
}

// DeleteRoute godoc
// @Summary      Route устгах
// @Tags         gateway
// @Produce      json
// @Param        id  path  string  true  "Route ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/routes/{id} [delete]
func (h Handler) DeleteRoute(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeleteRoute(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "route deleted successfully", nil)
}

func routeInput(req requests.GatewayRouteRequest) gatewayuc.RouteInput {
	return gatewayuc.RouteInput{
		ServiceID: req.ServiceID, Name: req.Name, Methods: req.Methods, Paths: req.Paths,
		StripPath: req.StripPath, PreserveHost: req.PreserveHost, Enabled: req.Enabled,
	}
}

// ── Consumers + keys ─────────────────────────────────────────────────────—

// ListConsumers godoc
// @Summary      Consumer-уудыг жагсаах
// @Tags         gateway
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/consumers [get]
func (h Handler) ListConsumers(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListConsumers(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "consumers fetched successfully", responses.ToGatewayConsumerList(list))
}

// CreateConsumer godoc
// @Summary      Consumer үүсгэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GatewayConsumerRequest  true  "Consumer"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gateway/consumers [post]
func (h Handler) CreateConsumer(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayConsumerRequest
	if !decode(w, r, &req) {
		return nil
	}
	c, err := h.usecase.CreateConsumer(r.Context(), consumerInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "consumer created successfully", responses.FromGatewayConsumer(c))
}

// UpdateConsumer godoc
// @Summary      Consumer шинэчлэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Consumer ID"
// @Param        body  body  requests.GatewayConsumerRequest  true  "Consumer"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/consumers/{id} [put]
func (h Handler) UpdateConsumer(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayConsumerRequest
	if !decode(w, r, &req) {
		return nil
	}
	c, err := h.usecase.UpdateConsumer(r.Context(), chi.URLParam(r, "id"), consumerInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "consumer updated successfully", responses.FromGatewayConsumer(c))
}

// DeleteConsumer godoc
// @Summary      Consumer устгах
// @Tags         gateway
// @Produce      json
// @Param        id  path  string  true  "Consumer ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/consumers/{id} [delete]
func (h Handler) DeleteConsumer(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeleteConsumer(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "consumer deleted successfully", nil)
}

func consumerInput(req requests.GatewayConsumerRequest) gatewayuc.ConsumerInput {
	return gatewayuc.ConsumerInput{
		Username: req.Username, CustomID: req.CustomID, Tags: req.Tags, Enabled: req.Enabled,
	}
}

// ListKeys godoc
// @Summary      Consumer-ийн API key-үүдийг жагсаах
// @Tags         gateway
// @Produce      json
// @Param        id  path  string  true  "Consumer ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/consumers/{id}/keys [get]
func (h Handler) ListKeys(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListKeys(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "keys fetched successfully", responses.ToGatewayKeyList(list))
}

// CreateKey godoc
// @Summary      Consumer-д шинэ API key үүсгэх (plaintext-ийг нэг удаа буцаана)
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Consumer ID"
// @Param        body  body  requests.GatewayKeyRequest  false  "Key"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gateway/consumers/{id}/keys [post]
func (h Handler) CreateKey(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayKeyRequest
	// Body нь сонголттой (хоосон body зөвшөөрнө) — алдаа гарвал зүгээр л
	// default-аар үүсгэнэ.
	_ = v1.DecodeBody(r, &req)
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	key, err := h.usecase.CreateKey(r.Context(), chi.URLParam(r, "id"),
		gatewayuc.KeyInput{Label: req.Label, ExpiresAt: req.ExpiresAt})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "api key created successfully", responses.FromGatewayKey(key))
}

// RevokeKey godoc
// @Summary      API key-г хүчингүй болгох (revoke)
// @Tags         gateway
// @Produce      json
// @Param        keyId  path  string  true  "Key ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/keys/{keyId}/revoke [post]
func (h Handler) RevokeKey(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.RevokeKey(r.Context(), chi.URLParam(r, "keyId")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "api key revoked successfully", nil)
}

// DeleteKey godoc
// @Summary      API key-г устгах
// @Tags         gateway
// @Produce      json
// @Param        keyId  path  string  true  "Key ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/keys/{keyId} [delete]
func (h Handler) DeleteKey(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeleteKey(r.Context(), chi.URLParam(r, "keyId")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "api key deleted successfully", nil)
}

// ── Policies ─────────────────────────────────────────────────────────────—

// ListPolicies godoc
// @Summary      Policy (plugin)-уудыг жагсаах
// @Tags         gateway
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/policies [get]
func (h Handler) ListPolicies(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListPolicies(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "policies fetched successfully", responses.ToGatewayPolicyList(list))
}

// CreatePolicy godoc
// @Summary      Policy үүсгэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GatewayPolicyRequest  true  "Policy"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gateway/policies [post]
func (h Handler) CreatePolicy(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayPolicyRequest
	if !decode(w, r, &req) {
		return nil
	}
	p, err := h.usecase.CreatePolicy(r.Context(), policyInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "policy created successfully", responses.FromGatewayPolicy(p))
}

// UpdatePolicy godoc
// @Summary      Policy шинэчлэх
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Policy ID"
// @Param        body  body  requests.GatewayPolicyRequest  true  "Policy"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/policies/{id} [put]
func (h Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) error {
	var req requests.GatewayPolicyRequest
	if !decode(w, r, &req) {
		return nil
	}
	p, err := h.usecase.UpdatePolicy(r.Context(), chi.URLParam(r, "id"), policyInput(req))
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "policy updated successfully", responses.FromGatewayPolicy(p))
}

// DeletePolicy godoc
// @Summary      Policy устгах
// @Tags         gateway
// @Produce      json
// @Param        id  path  string  true  "Policy ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gateway/policies/{id} [delete]
func (h Handler) DeletePolicy(w http.ResponseWriter, r *http.Request) error {
	if err := h.usecase.DeletePolicy(r.Context(), chi.URLParam(r, "id")); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "policy deleted successfully", nil)
}

func policyInput(req requests.GatewayPolicyRequest) gatewayuc.PolicyInput {
	return gatewayuc.PolicyInput{
		RouteID: req.RouteID, Type: req.Type, Config: req.Config, Enabled: req.Enabled,
	}
}
