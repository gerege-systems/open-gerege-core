// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gov нь иргэний "Төрийн үйлчилгээ" порталын /gov/* endpoint-уудыг
// үйлчилнэ. Каталогаас бусад нь баталгаажсан хэрэглэгчийн өгөгдөл тул userID-г
// токеноос (CurrentUser) авч usecase руу дамжуулна.
package gov

import (
	"net/http"

	govuc "template/internal/business/usecases/gov"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	usecase govuc.Usecase
}

func NewHandler(usecase govuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

// uid нь токеноос баталгаажсан хэрэглэгчийн ID-г авна.
func uid(r *http.Request) (string, bool) {
	u, err := httpauth.CurrentUserFromContext(r)
	if err != nil || u.ID == "" {
		return "", false
	}
	return u.ID, true
}

// pathID нь URL-ийн {id} параметрийг задалж хүчинтэй UUID эсэхийг шалгана. Буруу
// бол 400 бичээд false буцаана (дуудагч nil буцаана — Wrap дахин бичихгүй).
// Ингэснээр UUID биш id нь Postgres-ийн "invalid input syntax" 500-ийн оронд
// цэвэр 400 болно.
func pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		_ = v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid id")
		return "", false
	}
	return id, true
}

// ── Catalog + overview ────────────────────────────────────────────────────—

// ListServices godoc
// @Summary      Төрийн үйлчилгээний каталог
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/services [get]
func (h Handler) ListServices(w http.ResponseWriter, r *http.Request) error {
	list, err := h.usecase.ListServices(r.Context())
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "services fetched successfully", responses.ToGovServiceList(list))
}

// Overview godoc
// @Summary      Иргэний нүүр хуудасны нэгтгэл
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/overview [get]
func (h Handler) Overview(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	o, err := h.usecase.Overview(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "overview fetched successfully", responses.FromGovOverview(o))
}

// ── Applications ──────────────────────────────────────────────────────────—

// ListApplications godoc
// @Summary      Миний хүсэлтүүд
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/applications [get]
func (h Handler) ListApplications(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	list, err := h.usecase.ListApplications(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "applications fetched successfully", responses.ToGovApplicationList(list))
}

// Apply godoc
// @Summary      Үйлчилгээнд хүсэлт гаргах
// @Tags         gov
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GovApplyRequest  true  "Apply"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gov/applications [post]
func (h Handler) Apply(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	var req requests.GovApplyRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	app, err := h.usecase.Apply(r.Context(), id, govuc.ApplyRequest{ServiceID: req.ServiceID, Note: req.Note})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "application submitted successfully", responses.FromGovApplication(app))
}

// CancelApplication godoc
// @Summary      Хүсэлт цуцлах
// @Tags         gov
// @Produce      json
// @Param        id  path  string  true  "Application ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/applications/{id}/cancel [post]
func (h Handler) CancelApplication(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	pid, ok2 := pathID(w, r)
	if !ok2 {
		return nil
	}
	if err := h.usecase.CancelApplication(r.Context(), id, pid); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "application cancelled successfully", nil)
}

// ── References ────────────────────────────────────────────────────────────—

// ListReferences godoc
// @Summary      Миний лавлагаа
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/references [get]
func (h Handler) ListReferences(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	list, err := h.usecase.ListReferences(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "references fetched successfully", responses.ToGovReferenceList(list))
}

// RequestReference godoc
// @Summary      Лавлагаа захиалах
// @Tags         gov
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GovReferenceRequest  true  "Reference"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gov/references [post]
func (h Handler) RequestReference(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	var req requests.GovReferenceRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	ref, err := h.usecase.RequestReference(r.Context(), id, govuc.ReferenceRequest{Type: req.Type})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "reference issued successfully", responses.FromGovReference(ref))
}

// ── Notifications ─────────────────────────────────────────────────────────—

// ListNotifications godoc
// @Summary      Мэдэгдлүүд
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/notifications [get]
func (h Handler) ListNotifications(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	list, err := h.usecase.ListNotifications(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "notifications fetched successfully", responses.ToGovNotificationList(list))
}

// MarkNotificationRead godoc
// @Summary      Мэдэгдлийг уншсан болгох
// @Tags         gov
// @Produce      json
// @Param        id  path  string  true  "Notification ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/notifications/{id}/read [post]
func (h Handler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	pid, ok2 := pathID(w, r)
	if !ok2 {
		return nil
	}
	if err := h.usecase.MarkNotificationRead(r.Context(), id, pid); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "notification marked read", nil)
}

// MarkAllRead godoc
// @Summary      Бүх мэдэгдлийг уншсан болгох
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/notifications/read-all [post]
func (h Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	if err := h.usecase.MarkAllNotificationsRead(r.Context(), id); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "all notifications marked read", nil)
}

// ── Payments ──────────────────────────────────────────────────────────────—

// ListPayments godoc
// @Summary      Төлбөрүүд
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/payments [get]
func (h Handler) ListPayments(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	list, err := h.usecase.ListPayments(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "payments fetched successfully", responses.ToGovPaymentList(list))
}

// PayPayment godoc
// @Summary      Төлбөр төлөх
// @Tags         gov
// @Produce      json
// @Param        id  path  string  true  "Payment ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/payments/{id}/pay [post]
func (h Handler) PayPayment(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	pid, ok2 := pathID(w, r)
	if !ok2 {
		return nil
	}
	if err := h.usecase.PayPayment(r.Context(), id, pid); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "payment completed successfully", nil)
}

// ── Appointments ──────────────────────────────────────────────────────────—

// ListAppointments godoc
// @Summary      Цаг захиалгууд
// @Tags         gov
// @Produce      json
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/appointments [get]
func (h Handler) ListAppointments(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	list, err := h.usecase.ListAppointments(r.Context(), id)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "appointments fetched successfully", responses.ToGovAppointmentList(list))
}

// BookAppointment godoc
// @Summary      Цаг захиалах
// @Tags         gov
// @Accept       json
// @Produce      json
// @Param        body  body  requests.GovBookRequest  true  "Book"
// @Success      201  {object}  v1.BaseResponse
// @Router       /gov/appointments [post]
func (h Handler) BookAppointment(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	var req requests.GovBookRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	appt, err := h.usecase.BookAppointment(r.Context(), id, govuc.BookRequest{
		ServiceID: req.ServiceID, ScheduledAt: req.ScheduledAt, Location: req.Location, Note: req.Note,
	})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusCreated, "appointment booked successfully", responses.FromGovAppointment(appt))
}

// CancelAppointment godoc
// @Summary      Цаг захиалга цуцлах
// @Tags         gov
// @Produce      json
// @Param        id  path  string  true  "Appointment ID"
// @Success      200  {object}  v1.BaseResponse
// @Router       /gov/appointments/{id}/cancel [post]
func (h Handler) CancelAppointment(w http.ResponseWriter, r *http.Request) error {
	id, ok := uid(r)
	if !ok {
		return v1.NewAbortResponse(w, r, "invalid token")
	}
	pid, ok2 := pathID(w, r)
	if !ok2 {
		return nil
	}
	if err := h.usecase.CancelAppointment(r.Context(), id, pid); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "appointment cancelled successfully", nil)
}
