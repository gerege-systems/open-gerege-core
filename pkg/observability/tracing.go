// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package observability

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerName нь дуудагчид глобал provider-ээс tracer авахдаа ашиглах ёстой
// import зам бөгөөд ингэснээр энэ төслөөс ялгарах span бүр ижил
// instrumentation library шошготой болно.
const TracerName = "gerege-template"

// TracingConfig нь tracer-provider байгуулалтыг удирдана. Тэг утга нь no-op
// tracer (sampler=never, exporter байхгүй)-ийг өгдөг бөгөөд тест болон OTel
// env хувьсагчгүй `go run` нь анхдагчаар үүнийг авдаг.
type TracingConfig struct {
	// ServiceName нь service.name resource attribute болгон тохируулагдана.
	ServiceName string
	// Environment нь эцэстээ deployment.environment болдог.
	Environment string
	// Exporter нь зорилгыг сонгоно: "stdout" (dev), "otlp" (prod, OTLP/gRPC-ээр
	// OTEL_EXPORTER_OTLP_ENDPOINT руу), "" (идэвхгүй).
	Exporter string
	// SampleRatio нь head sampler-ийн харьцаа (0..1). 0 нь идэвхгүй болгоно, 1
	// нь бүгдийг тэмдэглэнэ. Production ихэвчлэн 0.01–0.1 дээр байдаг.
	SampleRatio float64
}

// Shutdown нь буферлэгдсэн span-уудыг гүйцээж гаргана. Үүнийг серверийн graceful
// shutdown (эвсэг унтраалт) дарааллд холбож, ингэснээр сүүлчийн span-ууд SIGTERM
// дээр алдагдахгүй.
type Shutdown func(context.Context) error

// SetupTracing нь глобал tracer provider + W3C trace-context propagator-ийг
// суулгаж, shutdown функц буцаана. cfg.Exporter хоосон үед дуудах газрууд
// nil-шалгалт хийх шаардлагагүй болгож no-op Shutdown буцаана.
func SetupTracing(ctx context.Context, cfg TracingConfig) (Shutdown, error) {
	if cfg.Exporter == "" {
		// Tracing идэвхгүй — глобал provider-ийг OTel-ийн noop анхдагч хэвээр
		// үлдээнэ, ингэснээр аливаа `tracer.Start(...)` дуудлага зүгээр л no-op
		// болно. Propagator-ийг гэсэн ч суулгана, ингэснээр дээд талын
		// traceparent толгой энэ сервисээр бүрэн бүтэн дамждаг.
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := buildExporter(ctx, cfg.Exporter)
	if err != nil {
		return nil, fmt.Errorf("build trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(orDefault(cfg.ServiceName, "go-rest-boilerplate")),
			semconv.DeploymentEnvironment(orDefault(cfg.Environment, "development")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(clampRatio(cfg.SampleRatio)))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// Tracer нь гар аргаар span ялгаруулахыг хүссэн дуудагчдад tracer буцаана.
// Үргэлж глобал provider-ээс татдаг тул SetupTracing-ийн сонголт төсөл даяар үйлчилнэ.
func Tracer() trace.Tracer { return otel.Tracer(TracerName) }

func buildExporter(ctx context.Context, kind string) (sdktrace.SpanExporter, error) {
	switch kind {
	case "stdout":
		// stdout руу цэвэрхэн хэвлэнэ. OTel collector-г локалаар ажиллуулах
		// шаардлагагүйгээр span-ууд ялгарч байгааг батлахад dev-д хэрэгтэй.
		return stdouttrace.New(stdouttrace.WithWriter(os.Stdout), stdouttrace.WithPrettyPrint())
	case "otlp":
		// Endpoint, толгойнууд, TLS гэх мэтийг otlptracegrpc нь стандарт
		// OTEL_EXPORTER_OTLP_* env хувьсагчдаас уншдаг — ингэснээр бид OTel SDK
		// аль хэдийн эзэмшдэг тохиргооны товчлууруудыг дахин зохиохгүй.
		return otlptracegrpc.New(ctx)
	default:
		return nil, fmt.Errorf("unknown exporter %q (expected stdout|otlp)", kind)
	}
}

func clampRatio(r float64) float64 {
	if r <= 0 {
		return 0
	}
	if r >= 1 {
		return 1
	}
	return r
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
