// Gerege Template Version 27.0
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

// GatewayRoute нь (methods, paths)-г нэг service рүү буулгана.
type GatewayRoute struct {
	ID           string
	ServiceID    string
	ServiceName  string // join-оор уншина (зөвхөн унших; INSERT/UPDATE-д хэрэглэхгүй)
	Name         string
	Methods      []string
	Paths        []string
	StripPath    bool
	PreserveHost bool
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}

// GatewayConsumer нь бүртгэлтэй API client.
type GatewayConsumer struct {
	ID        string
	Username  string
	CustomID  string
	Tags      []string
	Enabled   bool
	KeyCount  int // join-оор уншина (зөвхөн унших)
	CreatedAt time.Time
	UpdatedAt *time.Time
}

// GatewayAPIKey нь consumer-ийн credential. Зөвхөн SHA-256 hash хадгалагдана;
// Plaintext-ийг үүсгэх үед НЭГ удаа л буцаана (Plaintext талбар тэр үед л дүүрнэ).
type GatewayAPIKey struct {
	ID         string
	ConsumerID string
	Label      string
	Prefix     string
	Hash       string
	Plaintext  string // зөвхөн үүсгэх хариунд дүүрнэ; DB-д хадгалагдахгүй
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	Revoked    bool
	CreatedAt  time.Time
}

// GatewayPolicy нь route-д (route_id NULL бол global) хавсаргасан plugin.
type GatewayPolicy struct {
	ID        string
	RouteID   *string // nil = global
	RouteName string  // join-оор уншина
	Type      string
	Config    []byte // jsonb (raw JSON)
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt *time.Time
}

// GatewayRequestLog нь нэг proxy хүсэлтийн телеметр бичлэг.
type GatewayRequestLog struct {
	ID         string
	RouteID    *string
	RouteName  string
	ConsumerID *string
	Consumer   string
	Method     string
	Path       string
	Status     int
	LatencyMS  int
	ClientIP   string
	CreatedAt  time.Time
}

// GatewayOverview нь dashboard-ийн нэгтгэсэн статистик (сүүлийн 24 цаг).
type GatewayOverview struct {
	Services       int
	Routes         int
	Consumers      int
	ActiveKeys     int
	Requests24h    int
	Errors24h      int     // status >= 500
	RateLimited24h int     // status == 429
	ErrorRate      float64 // 0..1 (errors / requests)
	AvgLatencyMS   int
	P95LatencyMS   int
	StatusBuckets  []GatewayStatusBucket // 2xx/3xx/4xx/5xx тоо
	TopRoutes      []GatewayRouteStat
}

type GatewayStatusBucket struct {
	Class string // "2xx".."5xx"
	Count int
}

type GatewayRouteStat struct {
	RouteName string
	Count     int
}
