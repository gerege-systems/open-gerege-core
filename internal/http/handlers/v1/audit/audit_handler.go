// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package audit нь /v1/audit/* admin endpoint-уудыг үйлчилнэ — hash-chained
// audit log-ийг жагсаах болон гинжийн бүрэн бүтэн байдлыг шалгах. Бүх endpoint
// admin-only (route бүлэгт RequireAdmin суусан).
package audit

import (
	"net/http"
	"strconv"

	audituc "template/internal/business/usecases/audit"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
)

// Handler нь audit-домэйн endpoint-уудыг үйлчилнэ — зөвхөн audit.Usecase руу дууддаг.
type Handler struct {
	usecase audituc.Usecase
}

func NewHandler(usecase audituc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// defaultLimit / maxLimit нь жагсаалтын хуудаслалтын хязгаарууд.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// List godoc
// @Summary      Audit log жагсаах (admin)
// @Description  Hash-chained, append-only audit log-ийг хуудаслан буцаана. action / actor query параметрээр шүүж болно. Зөвхөн admin.
// @Tags         audit
// @Produce      json
// @Security     BearerAuth
// @Param        action  query     string  false  "action-аар шүүх"
// @Param        actor   query     string  false  "actor_user_id-аар шүүх (uuid)"
// @Param        limit   query     int     false  "хуудасны хэмжээ (default 50, max 200)"
// @Param        offset  query     int     false  "эхлэх байрлал (default 0)"
// @Success      200  {object}  v1.BaseResponse{data=[]responses.AuditLogResponse}  "Audit entries"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not an admin"
// @Router       /v1/audit [get]
func (h Handler) List(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	filter := repointerface.AuditListFilter{
		Action:      q.Get("action"),
		ActorUserID: q.Get("actor"),
	}
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

	rows, err := h.usecase.ListEvents(r.Context(), filter, limit, offset)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "audit entries fetched successfully", responses.ToAuditList(rows))
}

// Verify godoc
// @Summary      Audit гинж шалгах (admin)
// @Description  Audit log-ийн hash гинжийг genesis-ээс эхлэн дахин тооцоолж бүрэн бүтэн эсэхийг шалгана. ok=false бол broken_id нь эвдэрсэн эхний мөрийн id. Зөвхөн admin.
// @Tags         audit
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  v1.BaseResponse{data=responses.AuditVerifyResponse}  "Chain integrity status"
// @Failure      401  {object}  v1.BaseResponse  "Missing or invalid token"
// @Failure      403  {object}  v1.BaseResponse  "Not an admin"
// @Router       /v1/audit/verify [get]
func (h Handler) Verify(w http.ResponseWriter, r *http.Request) error {
	res, err := h.usecase.VerifyChain(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "audit chain verified", responses.AuditVerifyResponse{
		OK:       res.OK,
		BrokenID: res.BrokenID,
	})
}

// parseIntDefault нь query параметрийг int болгож задлана; хоосон/буруу бол def.
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
