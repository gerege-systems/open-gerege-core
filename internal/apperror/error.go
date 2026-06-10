// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package apperror нь business давхарга HTTP handler руу дамжуулдаг
// төрөлжсөн алдааны бүрхүүлийн (envelope) албан ёсны байршил юм. HTTP
// давхарга нь Type бүрийг статус код руу буулгадаг; usecase-ууд дотоод
// мэдээллийг клиент рүү алдалгүйгээр логдох зорилгоор Cause-ийг хавсаргадаг.
package apperror

// ErrorType нь domain алдааны ангиллыг илэрхийлнэ.
type ErrorType int

const (
	ErrTypeInternal ErrorType = iota
	ErrTypeNotFound
	ErrTypeUnauthorized
	ErrTypeForbidden
	ErrTypeConflict
	ErrTypeBadRequest
)

// DomainError нь business давхаргад дамждаг төрөлжсөн алдаа юм.
// Cause нь анхны алдааг хадгалдаг тул errors.Is / errors.As нь cause-ийн
// текстийг клиентийн хариу руу алдалгүйгээр боож хийсэн cause-уудад (жишээ нь
// sql.ErrNoRows, ctx-ийн алдаа) хүрч чадна.
type DomainError struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (e *DomainError) Error() string { return e.Message }

// Unwrap нь errors.Is / errors.As-д cause-ийн гинжээр явах боломжийг олгоно.
func (e *DomainError) Unwrap() error { return e.Cause }

// New нь өгөгдсөн төрөл болон мессежтэй DomainError үүсгэнэ.
func New(errType ErrorType, message string) *DomainError {
	return &DomainError{Type: errType, Message: message}
}

// Wrap нь одоо байгаа DomainError-ийн мессежийг өөрчлөхгүйгээр түүнд доод
// түвшний cause-ийг хавсаргана. Шинэ утга буцаана (оролтыг өөрчлөхгүй).
func Wrap(err *DomainError, cause error) *DomainError {
	if err == nil {
		return nil
	}
	return &DomainError{Type: err.Type, Message: err.Message, Cause: cause}
}

// Түгээмэл domain алдаануудад зориулсан хялбар constructor-ууд.

func NotFound(msg string) *DomainError     { return New(ErrTypeNotFound, msg) }
func Unauthorized(msg string) *DomainError { return New(ErrTypeUnauthorized, msg) }
func Forbidden(msg string) *DomainError    { return New(ErrTypeForbidden, msg) }
func Conflict(msg string) *DomainError     { return New(ErrTypeConflict, msg) }
func BadRequest(msg string) *DomainError   { return New(ErrTypeBadRequest, msg) }
func Internal(msg string) *DomainError     { return New(ErrTypeInternal, msg) }

// InternalCause нь тогтсон, ерөнхий, хэрэглэгчид харагдах мессежтэй дотоод
// алдаа үүсгэж, бодит cause-ийг логдох зорилгоор хадгалдаг. Доод түвшний алдаа
// 500 хариу болж хувирах байсан газар бүрд үүнийг ашигла — дотоод/library-ийн
// мессеж клиент рүү алдагдах ёсгүй.
func InternalCause(cause error) *DomainError {
	return &DomainError{
		Type:    ErrTypeInternal,
		Message: "internal server error",
		Cause:   cause,
	}
}
