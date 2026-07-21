// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"encoding/json"
	"net/http"

	authuc "template/internal/business/usecases/auth"
	"template/internal/datasources/rls"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/audit"
	"template/pkg/logger"
	"template/pkg/validators"
)

// EIDStart godoc
// @Summary      eID нэвтрэлт эхлүүлэх (QR / deep-link)
// @Description  Гадаад eID identity provider дээр QR/deep-link нэвтрэлтийг эхлүүлж, клиент харуулах session мэдээллийг (session_id, device_link_url, verification_code, expires_at) буцаана. Дараа нь /auth/eid/poll руу session_id-г дамжуулж төлвийг асууна.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  v1.BaseResponse{data=responses.EIDStartResponse}  "eID session started"
// @Failure      500  {object}  v1.BaseResponse                                   "Failed to reach eID provider"
// @Router       /auth/eid/start [post]
func (h Handler) EIDStart(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "EIDStart"
		fileName       = "auth_eid.go"
	)
	ctx := r.Context()

	// callbackUrl (сонголт): SAME-DEVICE (mobile browser) үед frontend <origin>/auth/eid/callback
	// дамжуулна; хоосон/байхгүй бол CROSS-DEVICE (desktop QR). Body байхгүй ч зүгээр (cross-device) —
	// декод алдааг үл хайхарч callbackUrl-ийг хоосон гэж үзнэ.
	var body struct {
		CallbackURL string `json:"callbackUrl"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	result, err := h.usecase.EIDStart(ctx, body.CallbackURL)
	if err != nil {
		logger.ErrorWithContext(ctx, "EIDStart failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session started", responses.FromEIDStartResponse(result))
}

// EIDStartByNationalID godoc
// @Summary      eID нэвтрэлт эхлүүлэх (РД-аар push)
// @Description  Иргэний РД (national_id)-аар нэвтрэлтийг эхлүүлж, тухайн РД-тэй холбоотой бүртгэлтэй төхөөрөмж рүү баталгаажуулах prompt push хийлгэнэ. QR/device_link шаардлагагүй тул зөвхөн session_id, verification_code, expires_at буцна. Дараа нь /auth/eid/poll руу session_id-г дамжуулж төлвийг асууна.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.EIDStartByNationalIDRequest  true  "Иргэний РД"
// @Success      200      {object}  v1.BaseResponse{data=responses.EIDStartResponse}  "eID session started"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body or missing national_id"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      500      {object}  v1.BaseResponse  "Failed to reach eID provider"
// @Router       /auth/eid/start-id [post]
func (h Handler) EIDStartByNationalID(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "EIDStartByNationalID"
		fileName       = "auth_eid.go"
	)
	ctx := r.Context()
	var req requests.EIDStartByNationalIDRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "EIDStartByNationalID: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "EIDStartByNationalID: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.EIDStartByNationalID(ctx, req.NationalID, req.CallbackUrl)
	if err != nil {
		logger.ErrorWithContext(ctx, "EIDStartByNationalID failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session started", responses.FromEIDStartResponse(result))
}

// EIDPoll godoc
// @Summary      eID session-ийн төлвийг асуух (long-poll)
// @Description  session_id-ийн төлвийг IdP-ээс long-poll-оор (≤25с) асууна. state нь RUNNING/COMPLETE/EXPIRED/REFUSED. COMPLETE үед identity-аар хэрэглэгчийг бүртгэж/шинэчилж, access+refresh токен хосыг /login-той ижил хэлбэрээр буцаана.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requests.EIDPollRequest  true  "eID session id"
// @Success      200      {object}  v1.BaseResponse{data=responses.EIDPollResponse}  "Session state (with tokens if COMPLETE)"
// @Failure      400      {object}  v1.BaseResponse  "Malformed JSON body or missing session_id"
// @Failure      422      {object}  v1.BaseResponse  "Validation error"
// @Failure      500      {object}  v1.BaseResponse  "Failed to reach eID provider"
// @Router       /auth/eid/poll [post]
func (h Handler) EIDPoll(w http.ResponseWriter, r *http.Request) error {
	const (
		controllerName = "auth"
		funcName       = "EIDPoll"
		fileName       = "auth_eid.go"
	)
	ctx := r.Context()
	var req requests.EIDPollRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		logger.WarnWithContext(ctx, "EIDPoll: invalid request body", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		logger.WarnWithContext(ctx, "EIDPoll: validation error", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	result, err := h.usecase.EIDPoll(ctx, authuc.EIDPollRequest{SessionID: req.SessionID, GoogleLinkToken: req.GoogleLinkToken})
	if err != nil {
		logger.ErrorWithContext(ctx, "EIDPoll failed in controller", logger.Fields{
			"controller": controllerName,
			"method":     funcName,
			"file":       fileName,
			"error":      err.Error(),
		})
		return v1.RespondWithError(w, r, err)
	}

	// COMPLETE үед л шинэ session үүссэн — нэвтрэлтийн амжилтыг audit-д тэмдэглэнэ.
	if result.State == "COMPLETE" {
		ev := auditFromRequest(r)
		ev.Type = audit.EventLoginSuccess
		ev.Success = true
		ev.UserID = result.User.ID
		audit.Record(ev)

		// Persisted hash-chained audit log руу best-effort бичнэ — амжилтгүй
		// болсон ч нэвтрэлтийн урсгалыг ХЭЗЭЭ Ч эвдэхгүй (зөвхөн log). actor нь
		// шинээр нэвтэрсэн хэрэглэгч тул ctx-д тухайн user identity-г суулгаж,
		// RecordEvent actor-г уншина.
		if h.auditUC != nil {
			actorCtx := rls.WithUser(ctx, result.User.ID)
			if auditErr := h.auditUC.RecordEvent(actorCtx, "auth.eid.login", "auth", result.User.ID, map[string]any{
				"method": "eid",
			}); auditErr != nil {
				logger.ErrorWithContext(ctx, "EIDPoll: persisted audit write failed (non-fatal)", logger.Fields{
					"controller": controllerName,
					"method":     funcName,
					"file":       fileName,
					"step":       "audit_record",
					"error":      auditErr.Error(),
				})
			}
		}
	}

	return v1.NewSuccessResponse(w, r, http.StatusOK, "eid session state", responses.FromEIDPollResponse(result))
}
