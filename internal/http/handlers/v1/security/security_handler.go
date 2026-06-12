// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package security нь /v1/security/events endpoint-уудыг үйлчилнэ — RASP-style
// security event-ийг хүлээн авах (нэвтэрсэн хэрэглэгч бүрт) болон жагсаах
// (admin-only). Клиентийн IP-г X-Forwarded-For / RemoteAddr-аас, user-agent-г
// header-ээс гаргана.
package security

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	securityuc "template/internal/business/usecases/security"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Handler нь security-домэйн endpoint-уудыг үйлчилнэ — зөвхөн security.Usecase руу дууддаг.
type Handler struct {
	usecase securityuc.Usecase
}

func NewHandler(usecase securityuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// Ingest godoc
// @Summary      Security event илгээх
// @Description  Нэвтэрсэн хэрэглэгч RASP-style security event илгээнэ (jailbreak, integrity, anomaly г.м.). Клиентийн IP + user-agent-г сервер тэмдэглэнэ. Хэрэглэгч зөвхөн өөрийнхөө тухай event илгээнэ.
// @Tags         security
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      requests.IngestSecurityEventRequest  true  "Security event"
// @Success      202  {object}  v1.BaseResponse  "Event recorded"
// @Failure      400  {object}  v1.BaseResponse  "Invalid request body"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Router       /v1/security/events [post]
func (h Handler) Ingest(w http.ResponseWriter, r *http.Request) error {
	user, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, err.Error())
	}
	var req requests.IngestSecurityEventRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	if err := h.usecase.Ingest(r.Context(), securityuc.IngestRequest{
		UserID:    user.ID,
		Kind:      req.Kind,
		Severity:  req.Severity,
		Source:    req.Source,
		UserAgent: r.Header.Get("User-Agent"),
		IP:        clientIP(r),
		Detail:    req.Detail,
	}); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusAccepted, "security event recorded", nil)
}

// List godoc
// @Summary      Security event жагсаах (admin)
// @Description  RASP-style security event-үүдийг хуудаслан буцаана. Зөвхөн admin.
// @Tags         security
// @Produce      json
// @Security     BearerAuth
// @Param        limit   query     int  false  "хуудасны хэмжээ (default 50, max 200)"
// @Param        offset  query     int  false  "эхлэх байрлал (default 0)"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.SecurityEventResponse}  "Security events"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not an admin"
// @Router       /v1/security/events [get]
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), defaultLimit)
	if limit > maxLimit {
		limit = maxLimit
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	offset := parseIntDefault(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	rows, err := h.usecase.List(r.Context(), limit, offset)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "security events fetched successfully", responses.ToSecurityEventList(rows))
}

// clientIP нь хүсэлтийн клиентийн IP-г тогтооно (auth_audit.go-той ижил энгийн
// арга): эхлээд X-Forwarded-For-ийн эхний хаяг, байхгүй бол RemoteAddr-ийн host.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
