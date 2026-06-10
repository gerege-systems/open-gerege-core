// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package logger

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger нь үндсэн лог бичих сан болгон zap-г ашиглан Logger interface-ийг
// хэрэгжүүлдэг. Хэрэглээний өөр өөр загваруудад логийн бичлэг дэх дуудагчийн
// мэдээллийг үнэн зөв байлгахын тулд оновчтой caller skip утгуудтай тусдаа
// logger instance-уудыг хадгалдаг.
type zapLogger struct {
	base          *zap.Logger // Шууд аргын дуудлагад зориулсан Logger
	contextLogger *zap.Logger // Context-ийг мэддэг аргын дуудлагад зориулсан Logger
	chainedLogger *zap.Logger // Гинжлэгдсэн (chained) аргын дуудлагад зориулсан Logger
	fields        []zap.Field // Хуримтлагдсан бүтэцлэгдсэн талбарууд
	isChained     bool        // Logger нь WithContext/WithFields-ээс буцаагдсан эсэхийг заана
}

// newZapLogger нь шинэ zap logger instance үүсгэж тохируулна.
// Энэ нь console болон/эсвэл файлын гаралтыг JSON эсвэл console encoder-аар
// эхлүүлж, логийн бичлэг дэх дуудагчийн мэдээллийг үнэн зөв байлгахын тулд
// тохирох caller skip утгуудтай олон logger instance-ийг тохируулна.
func newZapLogger(config Config) (Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "function",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if config.ConsoleJSONFormat {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	level := parseZapLevel(config.Level)
	cores := []zapcore.Core{}

	if config.EnableConsole {
		writer := zapcore.Lock(os.Stdout)
		cores = append(cores, zapcore.NewCore(encoder, writer, level))
	}

	if config.EnableFile && config.FileLocation != "" {
		file, err := os.OpenFile(config.FileLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		var fileEncoder zapcore.Encoder
		if config.FileJSONFormat {
			fileEncoder = zapcore.NewJSONEncoder(encoderConfig)
		} else {
			fileEncoder = zapcore.NewConsoleEncoder(encoderConfig)
		}

		writer := zapcore.AddSync(file)
		cores = append(cores, zapcore.NewCore(fileEncoder, writer, level))
	}

	if len(cores) == 0 {
		return nil, fmt.Errorf("no output configured (console or file must be enabled)")
	}

	core := zapcore.NewTee(cores...)

	baseLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(2),
	)

	contextLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(2),
	)

	chainedLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)

	if config.AppName != "" {
		baseLogger = baseLogger.With(zap.String("service", config.AppName))
		contextLogger = contextLogger.With(zap.String("service", config.AppName))
		chainedLogger = chainedLogger.With(zap.String("service", config.AppName))
	}

	return &zapLogger{
		base:          baseLogger,
		contextLogger: contextLogger,
		chainedLogger: chainedLogger,
		fields:        []zap.Field{},
		isChained:     false,
	}, nil
}

// parseZapLevel нь тэмдэгт мөр хэлбэрийн лог түвшинг zapcore.Level рүү хөрвүүлнэ.
// Танигдаагүй түвшний хувьд анхдагчаар InfoLevel-ийг буцаана.
func parseZapLevel(level string) zapcore.Level {
	switch level {
	case LevelDebug:
		return zapcore.DebugLevel
	case LevelInfo:
		return zapcore.InfoLevel
	case LevelWarn:
		return zapcore.WarnLevel
	case LevelError:
		return zapcore.ErrorLevel
	case LevelFatal:
		return zapcore.FatalLevel
	case LevelPanic:
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

// mergeFields нь хуримтлагдсан талбаруудыг аргын дуудлагаас ирсэн нэмэлт талбаруудтай нэгтгэнэ.
func (l *zapLogger) mergeFields(additional ...Fields) []zap.Field {
	totalSize := len(l.fields)
	for _, fieldMap := range additional {
		totalSize += len(fieldMap)
	}

	fields := make([]zap.Field, 0, totalSize)
	fields = append(fields, l.fields...)

	for _, fieldMap := range additional {
		for k, v := range fieldMap {
			fields = append(fields, zap.Any(k, v))
		}
	}

	return fields
}

// appendCorrelation нь ctx-ээс холбоосын ID-уудыг гаргаж аваад тэдгээрийг
// zap талбарууд болгон нэмнэ. Аливаа *WithContext арга үүнийг дууддаг тул
// логийн бичлэгүүд traceId (сервис хоорондын холбоос) + request_id (клиент рүү харсан)-г хуваалцдаг.
func appendCorrelation(ctx context.Context, fields []zap.Field) []zap.Field {
	if traceID := GetTraceIDFromContext(ctx); traceID != "" && traceID != "unknown" {
		fields = append(fields, zap.String("traceId", traceID))
	}
	if requestID := GetRequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	return fields
}

// Үндсэн лог бичих аргууд

// Debug нь сонголтот бүтэцлэгдсэн талбаруудтай debug түвшний мессеж бичнэ.
func (l *zapLogger) Debug(msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	if l.isChained {
		l.chainedLogger.Debug(msg, allFields...)
	} else {
		l.base.Debug(msg, allFields...)
	}
}

// Info нь сонголтот бүтэцлэгдсэн талбаруудтай info түвшний мессеж бичнэ.
func (l *zapLogger) Info(msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	if l.isChained {
		l.chainedLogger.Info(msg, allFields...)
	} else {
		l.base.Info(msg, allFields...)
	}
}

// Warn нь сонголтот бүтэцлэгдсэн талбаруудтай анхааруулга түвшний мессеж бичнэ.
func (l *zapLogger) Warn(msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	if l.isChained {
		l.chainedLogger.Warn(msg, allFields...)
	} else {
		l.base.Warn(msg, allFields...)
	}
}

// Error нь сонголтот бүтэцлэгдсэн талбаруудтай алдааны түвшний мессеж бичнэ.
func (l *zapLogger) Error(msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	if l.isChained {
		l.chainedLogger.Error(msg, allFields...)
	} else {
		l.base.Error(msg, allFields...)
	}
}

// Fatal нь сонголтот бүтэцлэгдсэн талбаруудтай fatal түвшний мессеж бичээд программаас гарна.
func (l *zapLogger) Fatal(msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	if l.isChained {
		l.chainedLogger.Fatal(msg, allFields...)
	} else {
		l.base.Fatal(msg, allFields...)
	}
}

// Panic нь сонголтот бүтэцлэгдсэн талбаруудтай panic түвшний мессеж бичээд panic хийнэ.
func (l *zapLogger) Panic(msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	if l.isChained {
		l.chainedLogger.Panic(msg, allFields...)
	} else {
		l.base.Panic(msg, allFields...)
	}
}

// Форматтай лог бичих аргууд

// Debugf нь хуримтлагдсан талбаруудыг ашиглан форматтай debug түвшний мессеж бичнэ.
func (l *zapLogger) Debugf(format string, args ...interface{}) {
	if l.isChained {
		l.chainedLogger.Debug(fmt.Sprintf(format, args...), l.fields...)
	} else {
		l.base.Debug(fmt.Sprintf(format, args...), l.fields...)
	}
}

// Infof нь хуримтлагдсан талбаруудыг ашиглан форматтай info түвшний мессеж бичнэ.
func (l *zapLogger) Infof(format string, args ...interface{}) {
	if l.isChained {
		l.chainedLogger.Info(fmt.Sprintf(format, args...), l.fields...)
	} else {
		l.base.Info(fmt.Sprintf(format, args...), l.fields...)
	}
}

// Warnf нь хуримтлагдсан талбаруудыг ашиглан форматтай анхааруулга түвшний мессеж бичнэ.
func (l *zapLogger) Warnf(format string, args ...interface{}) {
	if l.isChained {
		l.chainedLogger.Warn(fmt.Sprintf(format, args...), l.fields...)
	} else {
		l.base.Warn(fmt.Sprintf(format, args...), l.fields...)
	}
}

// Errorf нь хуримтлагдсан талбаруудыг ашиглан форматтай алдааны түвшний мессеж бичнэ.
func (l *zapLogger) Errorf(format string, args ...interface{}) {
	if l.isChained {
		l.chainedLogger.Error(fmt.Sprintf(format, args...), l.fields...)
	} else {
		l.base.Error(fmt.Sprintf(format, args...), l.fields...)
	}
}

// Fatalf нь хуримтлагдсан талбаруудыг ашиглан форматтай fatal түвшний мессеж бичээд программаас гарна.
func (l *zapLogger) Fatalf(format string, args ...interface{}) {
	if l.isChained {
		l.chainedLogger.Fatal(fmt.Sprintf(format, args...), l.fields...)
	} else {
		l.base.Fatal(fmt.Sprintf(format, args...), l.fields...)
	}
}

// Panicf нь хуримтлагдсан талбаруудыг ашиглан форматтай panic түвшний мессеж бичээд panic хийнэ.
func (l *zapLogger) Panicf(format string, args ...interface{}) {
	if l.isChained {
		l.chainedLogger.Panic(fmt.Sprintf(format, args...), l.fields...)
	} else {
		l.base.Panic(fmt.Sprintf(format, args...), l.fields...)
	}
}

// Context-ийг мэддэг лог бичих аргууд

// DebugWithContext нь context болон сонголтот бүтэцлэгдсэн талбаруудтай debug түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) DebugWithContext(ctx context.Context, msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	allFields = appendCorrelation(ctx, allFields)
	l.contextLogger.Debug(msg, allFields...)
}

// InfoWithContext нь context болон сонголтот бүтэцлэгдсэн талбаруудтай info түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) InfoWithContext(ctx context.Context, msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	allFields = appendCorrelation(ctx, allFields)
	l.contextLogger.Info(msg, allFields...)
}

// WarnWithContext нь context болон сонголтот бүтэцлэгдсэн талбаруудтай анхааруулга түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) WarnWithContext(ctx context.Context, msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	allFields = appendCorrelation(ctx, allFields)
	l.contextLogger.Warn(msg, allFields...)
}

// ErrorWithContext нь context болон сонголтот бүтэцлэгдсэн талбаруудтай алдааны түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) ErrorWithContext(ctx context.Context, msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	allFields = appendCorrelation(ctx, allFields)
	l.contextLogger.Error(msg, allFields...)
}

// FatalWithContext нь context болон сонголтот бүтэцлэгдсэн талбаруудтай fatal түвшний мессеж бичээд гарна.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) FatalWithContext(ctx context.Context, msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	allFields = appendCorrelation(ctx, allFields)
	l.contextLogger.Fatal(msg, allFields...)
}

// PanicWithContext нь context болон сонголтот бүтэцлэгдсэн талбаруудтай panic түвшний мессеж бичээд panic хийнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) PanicWithContext(ctx context.Context, msg string, fields ...Fields) {
	allFields := l.mergeFields(fields...)
	allFields = appendCorrelation(ctx, allFields)
	l.contextLogger.Panic(msg, allFields...)
}

// Context-ийг мэддэг форматтай лог бичих аргууд

// DebugfWithContext нь context-той форматтай debug түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) DebugfWithContext(ctx context.Context, format string, args ...interface{}) {
	fields := make([]zap.Field, len(l.fields))
	copy(fields, l.fields)
	fields = appendCorrelation(ctx, fields)

	l.contextLogger.Debug(fmt.Sprintf(format, args...), fields...)
}

// InfofWithContext нь context-той форматтай info түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) InfofWithContext(ctx context.Context, format string, args ...interface{}) {
	fields := make([]zap.Field, len(l.fields))
	copy(fields, l.fields)
	fields = appendCorrelation(ctx, fields)

	l.contextLogger.Info(fmt.Sprintf(format, args...), fields...)
}

// WarnfWithContext нь context-той форматтай анхааруулга түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) WarnfWithContext(ctx context.Context, format string, args ...interface{}) {
	fields := make([]zap.Field, len(l.fields))
	copy(fields, l.fields)
	fields = appendCorrelation(ctx, fields)

	l.contextLogger.Warn(fmt.Sprintf(format, args...), fields...)
}

// ErrorfWithContext нь context-той форматтай алдааны түвшний мессеж бичнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) ErrorfWithContext(ctx context.Context, format string, args ...interface{}) {
	fields := make([]zap.Field, len(l.fields))
	copy(fields, l.fields)
	fields = appendCorrelation(ctx, fields)

	l.contextLogger.Error(fmt.Sprintf(format, args...), fields...)
}

// FatalfWithContext нь context-той форматтай fatal түвшний мессеж бичээд гарна.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) FatalfWithContext(ctx context.Context, format string, args ...interface{}) {
	fields := make([]zap.Field, len(l.fields))
	copy(fields, l.fields)
	fields = appendCorrelation(ctx, fields)

	l.contextLogger.Fatal(fmt.Sprintf(format, args...), fields...)
}

// PanicfContext нь context-той форматтай panic түвшний мессеж бичээд panic хийнэ.
// Боломжтой бол context-оос traceId-г автоматаар гаргаж аваад оруулна.
func (l *zapLogger) PanicfContext(ctx context.Context, format string, args ...interface{}) {
	fields := make([]zap.Field, len(l.fields))
	copy(fields, l.fields)
	fields = appendCorrelation(ctx, fields)

	l.contextLogger.Panic(fmt.Sprintf(format, args...), fields...)
}

// Гинжлэх (chaining) аргууд

// WithFields нь өгөгдсөн талбаруудыг одоо байгаа хуримтлагдсан талбаруудтай
// нэгтгэсэн шинэ logger instance-ийг буцаана. Аргын гинжлэлийг (chaining) дэмжинэ.
func (l *zapLogger) WithFields(fields Fields) Logger {
	newFields := make([]zap.Field, 0, len(l.fields)+len(fields))
	newFields = append(newFields, l.fields...)

	for k, v := range fields {
		newFields = append(newFields, zap.Any(k, v))
	}

	return &zapLogger{
		base:          l.base,
		contextLogger: l.contextLogger,
		chainedLogger: l.chainedLogger,
		fields:        newFields,
		isChained:     true,
	}
}

// WithContext нь ctx-д байгаа тохиолдолд хоёр холбоосын ID-г (OTel-ийн traceId
// болон X-Request-ID-ийн request_id) шигтгэсэн шинэ logger instance-ийг буцаана.
// Аргын гинжлэлийг (chaining) дэмжинэ.
func (l *zapLogger) WithContext(ctx context.Context) Logger {
	newFields := make([]zap.Field, 0, len(l.fields)+2)
	newFields = append(newFields, l.fields...)
	newFields = appendCorrelation(ctx, newFields)

	return &zapLogger{
		base:          l.base,
		contextLogger: l.contextLogger,
		chainedLogger: l.chainedLogger,
		fields:        newFields,
		isChained:     true,
	}
}
