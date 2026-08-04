// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eidauth нь бүртгэлтэй апп (relying party)-д зориулсан eID
// НЭВТРЭЛТИЙН proxy-ийн handler. Дуудагч нь аппын OAuth токеноор
// (client_credentials) баталгаажна — иргэн энэ үед хараахан танигдаагүй тул
// иргэний токен байх боломжгүй.
package eidauth

import (
	"net/http"

	eidauthuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/eidauth"
	"github.com/gerege-systems/open-gerege-core/core/http/datatransfers/requests"
	"github.com/gerege-systems/open-gerege-core/core/http/datatransfers/responses"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/open-gerege-core/pkg/logger"
	"github.com/gerege-systems/open-gerege-core/pkg/validators"
)

const (
	controllerName = "eidauth"
	fileName       = "eidauth_handler.go"
)

// Handler нь eID нэвтрэлтийн proxy-ийн HTTP давхарга.
type Handler struct {
	usecase eidauthuc.Usecase
}

// NewHandler нь handler үүсгэнэ.
func NewHandler(usecase eidauthuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// Start godoc
// @Summary      eID нэвтрэлт эхлүүлэх (QR/device-link, RP proxy)
// @Description  Бүртгэлтэй апп (RP)-д зориулсан прокси: SSO нь ӨӨРИЙН eID RP креденшлээр QR/device-link нэвтрэлт эхлүүлж өгнө. Аппад eID креденшл хэрэггүй. Аппын OAuth токен (client_credentials) + svc:eid-auth эрх шаардана.
// @Tags         eid-auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.EIDAuthStartRequest  false  "Сонголтоор callbackUrl (same-device)"
// @Success      200      {object}  v1.BaseResponse{data=responses.EIDAuthStartResponse}  "eID session started"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body"
// @Failure      401      {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403      {object}  v1.BaseResponse  "Application is not granted this service"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      500      {object}  v1.BaseResponse  "Failed to reach eID provider"
// @Router       /eid-auth/start [post]
func (h Handler) Start(w http.ResponseWriter, r *http.Request) error {
	const funcName = "Start"
	ctx := r.Context()

	// Body нь сонголттой (зөвхөн callbackUrl) — хоосон бие зөвшөөрнө.
	var req requests.EIDAuthStartRequest
	if r.ContentLength > 0 {
		if err := v1.DecodeBody(r, &req); err != nil {
			logger.WarnWithContext(ctx, "eid-auth Start: invalid request body", logger.Fields{
				"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
			})
			return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
		}
		if err := validators.ValidatePayloads(req); err != nil {
			return v1.RespondWithError(w, r, err)
		}
	}

	result, err := h.usecase.Start(ctx, eidauthuc.StartRequest{CallbackURL: req.CallbackUrl})
	if err != nil {
		logger.ErrorWithContext(ctx, "eid-auth Start failed", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session started", responses.FromEIDAuthStart(result))
}

// StartByNationalID godoc
// @Summary      eID нэвтрэлт эхлүүлэх (РД-аар push, RP proxy)
// @Description  Бүртгэлтэй апп (RP)-д зориулсан прокси: иргэний РД-аар бүртгэлтэй төхөөрөмж рүү баталгаажуулах push илгээнэ. Аппын OAuth токен (client_credentials) + svc:eid-auth эрх шаардана.
// @Tags         eid-auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.EIDAuthStartByNationalIDRequest  true  "Иргэний РД"
// @Success      200      {object}  v1.BaseResponse{data=responses.EIDAuthStartResponse}  "eID session started"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body or unknown national_id"
// @Failure      401      {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403      {object}  v1.BaseResponse  "Application is not granted this service"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      500      {object}  v1.BaseResponse  "Failed to reach eID provider"
// @Router       /eid-auth/start-id [post]
func (h Handler) StartByNationalID(w http.ResponseWriter, r *http.Request) error {
	const funcName = "StartByNationalID"
	ctx := r.Context()

	var req requests.EIDAuthStartByNationalIDRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "eid-auth StartByNationalID: invalid request body", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.StartByNationalID(ctx, eidauthuc.StartByNationalIDRequest{
		NationalID:  req.NationalID,
		CallbackURL: req.CallbackUrl,
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "eid-auth StartByNationalID failed", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session started", responses.FromEIDAuthStart(result))
}

// Poll godoc
// @Summary      eID session-ий төлөв (RP proxy)
// @Description  eID session-ийн төлвийг long-poll-оор асууна. COMPLETE үед баталгаажсан иргэний identity-г буцаана — дуудагч апп өөрийн бодлогоор (нэвтрүүлэх/баталгаажуулах) шийднэ. Аппын OAuth токен (client_credentials) + svc:eid-auth эрх шаардана.
// @Tags         eid-auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      requests.EIDAuthPollRequest  true  "Session ID"
// @Success      200      {object}  v1.BaseResponse{data=responses.EIDAuthPollResponse}  "eID session state"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body or missing session_id"
// @Failure      401      {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403      {object}  v1.BaseResponse  "Application is not granted this service"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      500      {object}  v1.BaseResponse  "Failed to reach eID provider"
// @Router       /eid-auth/poll [post]
func (h Handler) Poll(w http.ResponseWriter, r *http.Request) error {
	const funcName = "Poll"
	ctx := r.Context()

	var req requests.EIDAuthPollRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "eid-auth Poll: invalid request body", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.Poll(ctx, eidauthuc.PollRequest{SessionID: req.SessionID})
	if err != nil {
		logger.ErrorWithContext(ctx, "eid-auth Poll failed", logger.Fields{
			"controller": controllerName, "method": funcName, "file": fileName, "error": err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session state", responses.FromEIDAuthPoll(result))
}
