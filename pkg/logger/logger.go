// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package logger

import (
	"context"
	"errors"
)

var (
	// ErrInvalidLoggerInstance нь буруу logger instance төрөл өгөгдсөн үед буцаагдана
	ErrInvalidLoggerInstance = errors.New("invalid logger instance")

	// defaultLogger нь глобал logger instance юм
	defaultLogger Logger
)

// Лог түвшний тогтмолууд
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
	LevelPanic = "panic"
)

// Logger instance-ийн төрлүүд
const (
	InstanceZap int = iota
	// Ирээдүйд logger-ийн өөр хэрэгжүүлэлтүүд
	// ...
	// InstanceZerolog
	// InstanceLogrus
)

// ctxKey нь context түлхүүрүүдэд зориулсан экспортлогдоогүй төрөл бөгөөд бусад
// package-ийн ашиглаж болзошгүй нүцгэн тэмдэг мөртэй мөргөлдөхөөс сэргийлнэ (staticcheck SA1029).
type ctxKey string

// Холбоосын ID-уудад зориулсан context түлхүүрүүд. TraceIDKey нь W3C trace
// ID-г хадгална (сервис хоорондын холбоосын тулд OTel автоматаар бөглөдөг);
// RequestIDKey нь клиент рүү буцааж цуурайтсан гадаад X-Request-ID-г хадгална.
// Аливаа *WithContext лог арга нь байгаа тохиолдолд хоёр талбарыг хоёуланг нь ялгаруулна.
const (
	TraceIDKey   ctxKey = "traceId"
	RequestIDKey ctxKey = "request_id"
)

// Fields нь бүтэцлэгдсэн логийн талбаруудыг илэрхийлнэ
type Fields map[string]interface{}

// Logger нь бүх logger хэрэгжүүлэлтэд зориулсан interface-ийг тодорхойлно
type Logger interface {
	// Үндсэн лог бичих аргууд
	Debug(msg string, fields ...Fields)
	Info(msg string, fields ...Fields)
	Warn(msg string, fields ...Fields)
	Error(msg string, fields ...Fields)
	Fatal(msg string, fields ...Fields)
	Panic(msg string, fields ...Fields)

	// Форматтай лог бичих аргууд
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	// Context-ийг мэддэг лог бичих аргууд
	DebugWithContext(ctx context.Context, msg string, fields ...Fields)
	InfoWithContext(ctx context.Context, msg string, fields ...Fields)
	WarnWithContext(ctx context.Context, msg string, fields ...Fields)
	ErrorWithContext(ctx context.Context, msg string, fields ...Fields)
	FatalWithContext(ctx context.Context, msg string, fields ...Fields)
	PanicWithContext(ctx context.Context, msg string, fields ...Fields)

	// Context-ийг мэддэг форматтай лог бичих аргууд
	DebugfWithContext(ctx context.Context, format string, args ...interface{})
	InfofWithContext(ctx context.Context, format string, args ...interface{})
	WarnfWithContext(ctx context.Context, format string, args ...interface{})
	ErrorfWithContext(ctx context.Context, format string, args ...interface{})
	FatalfWithContext(ctx context.Context, format string, args ...interface{})
	PanicfContext(ctx context.Context, format string, args ...interface{})

	// Гинжлэх (chaining) аргууд
	WithFields(fields Fields) Logger
	WithContext(ctx context.Context) Logger
}

// Config нь logger-ийн тохиргоог илэрхийлнэ
type Config struct {
	Level             string
	EnableConsole     bool
	ConsoleJSONFormat bool
	EnableFile        bool
	FileJSONFormat    bool
	FileLocation      string
	AppName           string
	SamplingEnabled   bool
}

// NewLogger нь өгөгдсөн тохиргоо болон төрөл дээр үндэслэн шинэ logger instance үүсгэнэ
func NewLogger(config Config, instanceType int) (Logger, error) {
	switch instanceType {
	case InstanceZap:
		return newZapLogger(config)
	default:
		return nil, ErrInvalidLoggerInstance
	}
}

// SetDefault нь анхдагч глобал logger instance-ийг тохируулна
func SetDefault(logger Logger) {
	defaultLogger = logger
}

// GetDefault нь анхдагч глобал logger instance-ийг буцаана
func GetDefault() Logger {
	return defaultLogger
}

// InitDefault нь анхдагч глобал logger-ийг эхлүүлнэ
func InitDefault(config Config, instanceType int) error {
	logger, err := NewLogger(config, instanceType)
	if err != nil {
		return err
	}
	SetDefault(logger)
	return nil
}

// Анхдагч logger-ийг ашигладаг глобал тохь тухтай функцууд

// Debug нь сонголтот талбаруудтай debug мессеж бичнэ
func Debug(msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.Debug(msg, fields...)
	}
}

// Info нь сонголтот талбаруудтай info мессеж бичнэ
func Info(msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.Info(msg, fields...)
	}
}

// Warn нь сонголтот талбаруудтай анхааруулга мессеж бичнэ
func Warn(msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.Warn(msg, fields...)
	}
}

// Error нь сонголтот талбаруудтай алдааны мессеж бичнэ
func Error(msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.Error(msg, fields...)
	}
}

// Fatal нь сонголтот талбаруудтай fatal мессеж бичээд гарна
func Fatal(msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.Fatal(msg, fields...)
	}
}

// Panic нь сонголтот талбаруудтай panic мессеж бичээд panic хийнэ
func Panic(msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.Panic(msg, fields...)
	}
}

// Debugf нь форматтай debug мессеж бичнэ
func Debugf(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debugf(format, args...)
	}
}

// Infof нь форматтай info мессеж бичнэ
func Infof(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Infof(format, args...)
	}
}

// Warnf нь форматтай анхааруулга мессеж бичнэ
func Warnf(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warnf(format, args...)
	}
}

// Errorf нь форматтай алдааны мессеж бичнэ
func Errorf(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Errorf(format, args...)
	}
}

// Fatalf нь форматтай fatal мессеж бичээд гарна
func Fatalf(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Fatalf(format, args...)
	}
}

// Panicf нь форматтай panic мессеж бичээд panic хийнэ
func Panicf(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Panicf(format, args...)
	}
}

// DebugWithContext нь context болон сонголтот талбаруудтай debug мессеж бичнэ
func DebugWithContext(ctx context.Context, msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.DebugWithContext(ctx, msg, fields...)
	}
}

// InfoWithContext нь context болон сонголтот талбаруудтай info мессеж бичнэ
func InfoWithContext(ctx context.Context, msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.InfoWithContext(ctx, msg, fields...)
	}
}

// WarnWithContext нь context болон сонголтот талбаруудтай анхааруулга мессеж бичнэ
func WarnWithContext(ctx context.Context, msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.WarnWithContext(ctx, msg, fields...)
	}
}

// ErrorWithContext нь context болон сонголтот талбаруудтай алдааны мессеж бичнэ
func ErrorWithContext(ctx context.Context, msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.ErrorWithContext(ctx, msg, fields...)
	}
}

// FatalWithContext нь context болон сонголтот талбаруудтай fatal мессеж бичээд гарна
func FatalWithContext(ctx context.Context, msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.FatalWithContext(ctx, msg, fields...)
	}
}

// PanicWithContext нь context болон сонголтот талбаруудтай panic мессеж бичээд panic хийнэ
func PanicWithContext(ctx context.Context, msg string, fields ...Fields) {
	if defaultLogger != nil {
		defaultLogger.PanicWithContext(ctx, msg, fields...)
	}
}

// DebugfWithContext нь context-той форматтай debug мессеж бичнэ
func DebugfWithContext(ctx context.Context, format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.DebugfWithContext(ctx, format, args...)
	}
}

// InfofWithContext нь context-той форматтай info мессеж бичнэ
func InfofWithContext(ctx context.Context, format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.InfofWithContext(ctx, format, args...)
	}
}

// WarnfWithContext нь context-той форматтай анхааруулга мессеж бичнэ
func WarnfWithContext(ctx context.Context, format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.WarnfWithContext(ctx, format, args...)
	}
}

// ErrorfWithContext нь context-той форматтай алдааны мессеж бичнэ
func ErrorfWithContext(ctx context.Context, format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.ErrorfWithContext(ctx, format, args...)
	}
}

// FatalfWithContext нь context-той форматтай fatal мессеж бичээд гарна
func FatalfWithContext(ctx context.Context, format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.FatalfWithContext(ctx, format, args...)
	}
}

// PanicfContext нь context-той форматтай panic мессеж бичээд panic хийнэ
func PanicfContext(ctx context.Context, format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.PanicfContext(ctx, format, args...)
	}
}

// WithFields нь өгөгдсөн талбаруудыг хавсаргасан logger-ийг буцаана
func WithFields(fields Fields) Logger {
	if defaultLogger != nil {
		return defaultLogger.WithFields(fields)
	}
	return nil
}

// WithContext нь context-той (байгаа бол trace ID-г оруулаад) logger-ийг буцаана
func WithContext(ctx context.Context) Logger {
	if defaultLogger != nil {
		return defaultLogger.WithContext(ctx)
	}
	return nil
}
