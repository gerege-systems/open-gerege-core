// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// API Gateway-ийн домэйн entity-үүд. Эдгээр нь gateway-ийн ТОХИРГОО ба
// телеметр (хэрэглэгч-тус-бүрийн биш) тул RLS-д хамаарахгүй — roles/permissions-
// тэй ижил ангилал. config/telemetry нь жинхэнэ proxy биш, харин admin UI-аар
// удирдагдах бүртгэл.

// PolicyType нь нэг route-д (эсвэл global) хавсаргах plugin-ий төрөл.
const (
	PolicyRateLimit  = "rate-limit"
	PolicyKeyAuth    = "key-auth"
	PolicyCORS       = "cors"
	PolicyIPRestrict = "ip-restriction"
	PolicyTransform  = "request-transform"
)

// GatewayService нь route-ууд proxy хийдэг upstream backend.
type GatewayService struct {
	ID             string
	Name           string
	Protocol       string
	Host           string
	Port           int
	Path           string
	Retries        int
	ConnectTimeout int // ms
	Tags           []string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      *time.Time
}

// GatewayRequestLog нь DAN backend руу ирсэн нэг бодит /api хүсэлтийн телеметр
// бичлэг (middleware бичнэ). Runtime proxy байхгүй тул route/consumer холбоосгүй.
type GatewayRequestLog struct {
	ID        string
	Method    string
	Path      string
	Status    int
	LatencyMS int
	ClientIP  string
	CreatedAt time.Time
}

// GatewayOverview нь dashboard-ийн нэгтгэсэн статистик (сүүлийн 24 цаг).
type GatewayOverview struct {
	Services       int
	Consumers      int // applications тоо
	ActiveKeys     int // application_services (service эрх) тоо
	Requests24h    int
	Errors24h      int     // status >= 500
	RateLimited24h int     // status == 429
	ErrorRate      float64 // 0..1 (errors / requests)
	AvgLatencyMS   int
	P95LatencyMS   int
	StatusBuckets  []GatewayStatusBucket // 2xx/3xx/4xx/5xx тоо
	TopPaths       []GatewayPathStat     // хамгийн их хүсэлттэй зам
}

type GatewayStatusBucket struct {
	Class string // "2xx".."5xx"
	Count int
}

// GatewayPathStat нь хамгийн их хүсэлттэй зам (хуучин TopRoutes-ыг орлоно).
type GatewayPathStat struct {
	Path  string
	Count int
}
