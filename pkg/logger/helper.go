// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package logger

import "context"

// GetTraceIDFromContext нь context-оос trace ID-г гаргаж авна
func GetTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		return traceID
	}

	return ""
}

// GetRequestIDFromContext нь context-оос гадаад X-Request-ID-г гаргаж авна.
// Хүсэлт түүнийг авч яваагүй бөгөөд middleware хараахан ажиллаагүй үед хоосон байна.
func GetRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		return requestID
	}
	return ""
}

// ConvertMapToFields нь map-ийг Fields рүү хөрвүүлнэ
func ConvertMapToFields(data map[string]interface{}) Fields {
	fields := make(Fields, len(data))
	for k, v := range data {
		fields[k] = v
	}
	return fields
}

// MergeFields нь олон Fields-г нэг болгон нэгтгэнэ
func MergeFields(fieldMaps ...Fields) Fields {
	totalSize := 0
	for _, f := range fieldMaps {
		totalSize += len(f)
	}

	result := make(Fields, totalSize)
	for _, f := range fieldMaps {
		for k, v := range f {
			result[k] = v
		}
	}
	return result
}
