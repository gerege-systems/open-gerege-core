// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package v1

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"template/internal/apperror"
	"template/internal/constants"
	"template/pkg/logger"
	"template/pkg/validators"
)

// maxBodyBytes нь DecodeBody-д уншиж болох JSON body-ийн дээд хэмжээ —
// route-түвшний BodySizeLimit middleware-ийн дээр гүний хамгаалалт.
const maxBodyBytes = 1 << 20 // 1 MiB

type BaseResponse struct {
	Status    bool        `json:"status"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// HandlerFunc нь алдаа буцаадаг handler юм — Wrap нь түүнийг стандарт
// http.HandlerFunc болгож, буцаасан алдааг нэгдсэн дугтуйгаар хариулна.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Wrap нь алдаа буцаадаг handler-г http.HandlerFunc болгоно. Handler-ууд
// доорх туслахуудаар (NewSuccessResponse/RespondWithError) хариугаа БИЧИЖ
// нил буцаадаг тул энд буцсан алдаа нь зөвхөн бичих/encode-ийн алдаа —
// log хийнэ (хариу аль хэдийн бичигдсэн тул дахин бичихгүй).
func Wrap(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			logger.ErrorWithContext(r.Context(), "response write failed", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryHTTP,
				"path":                   r.URL.Path,
				"error":                  err.Error(),
			})
		}
	}
}

// requestID нь request-id middleware-ийн context-д бичсэн корреляцийн
// ID-г уншина.
func requestID(r *http.Request) string {
	if v, ok := r.Context().Value(logger.RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// DecodeBody нь хүсэлтийн JSON body-г dst руу задлана. Танихгүй талбарыг
// татгалзаж, body-ийн хэмжээг хязгаарлана.
func DecodeBody(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, body BaseResponse) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(body)
}

// NewSuccessResponse нь амжилтын дугтуй бичнэ.
func NewSuccessResponse(w http.ResponseWriter, r *http.Request, statusCode int, message string, data interface{}) error {
	return writeJSON(w, statusCode, BaseResponse{
		Status:    true,
		Message:   message,
		Data:      data,
		RequestID: requestID(r),
	})
}

// NewErrorResponse нь алдааны дугтуй бичнэ.
func NewErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, errMsg string) error {
	return writeJSON(w, statusCode, BaseResponse{
		Status:    false,
		Message:   errMsg,
		RequestID: requestID(r),
	})
}

// NewAbortResponse нь нэгдсэн дугтуйтай 401-г үзүүлнэ (auth middleware
// ашигладаг).
func NewAbortResponse(w http.ResponseWriter, r *http.Request, message string) error {
	return NewErrorResponse(w, r, http.StatusUnauthorized, message)
}

// mapDomainErrorToHTTP нь домэйн алдааг HTTP статус код руу хувиргана.
func mapDomainErrorToHTTP(err error) int {
	var domErr *apperror.DomainError
	if errors.As(err, &domErr) {
		switch domErr.Type {
		case apperror.ErrTypeNotFound:
			return http.StatusNotFound
		case apperror.ErrTypeUnauthorized:
			return http.StatusUnauthorized
		case apperror.ErrTypeForbidden:
			return http.StatusForbidden
		case apperror.ErrTypeConflict:
			return http.StatusConflict
		case apperror.ErrTypeBadRequest:
			return http.StatusBadRequest
		default:
			return http.StatusInternalServerError
		}
	}
	return http.StatusInternalServerError
}

// RespondWithError нь цэвэрлэсэн алдааны хариу гаргаж, дотоод (5xx) бүх
// алдааны үндсэн шалтгааныг бүртгэдэг. "Дэлгэрэнгүйг бүртгэж, хэрэглэгчид
// цэвэр мессеж харуулах" дүрмийг төвлөрүүлдэг тул аль ч handler ороомог
// болсон сангийн алдааг body руу санамсаргүй түлхэхгүй.
func RespondWithError(w http.ResponseWriter, r *http.Request, err error) error {
	// Баталгаажуулалтын алдаанууд талбар бүрийн дэлгэрэнгүйтэйгээр 422
	// болж гарна.
	var ve *validators.ValidationErrors
	if errors.As(err, &ve) {
		return writeJSON(w, http.StatusUnprocessableEntity, BaseResponse{
			Status:    false,
			Message:   "validation failed",
			Data:      map[string]any{"errors": ve.Errors},
			RequestID: requestID(r),
		})
	}

	status := mapDomainErrorToHTTP(err)
	message := err.Error()

	if status >= http.StatusInternalServerError {
		fields := logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryHTTP,
			"path":                   r.URL.Path,
		}
		var domErr *apperror.DomainError
		if errors.As(err, &domErr) && domErr.Cause != nil {
			fields["cause"] = domErr.Cause.Error()
		} else {
			fields["cause"] = err.Error()
		}
		if rid := requestID(r); rid != "" {
			fields["request_id"] = rid
		}
		logger.Error("internal error while handling request", fields)
		message = "internal server error"
	}

	return NewErrorResponse(w, r, status, message)
}
