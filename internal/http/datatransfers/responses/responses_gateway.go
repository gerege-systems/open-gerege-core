// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"encoding/json"
	"time"

	"template/internal/business/domain"
)

// ── Services ────────────────────────────────────────────────────────────────

type GatewayServiceResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Protocol       string     `json:"protocol"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	Path           string     `json:"path"`
	Retries        int        `json:"retries"`
	ConnectTimeout int        `json:"connect_timeout_ms"`
	Tags           []string   `json:"tags"`
	Enabled        bool       `json:"enabled"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
}

func FromGatewayService(s domain.GatewayService) GatewayServiceResponse {
	return GatewayServiceResponse{
		ID: s.ID, Name: s.Name, Protocol: s.Protocol, Host: s.Host, Port: s.Port, Path: s.Path,
		Retries: s.Retries, ConnectTimeout: s.ConnectTimeout, Tags: nonNil(s.Tags),
		Enabled: s.Enabled, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func ToGatewayServiceList(list []domain.GatewayService) []GatewayServiceResponse {
	out := make([]GatewayServiceResponse, 0, len(list))
	for _, s := range list {
		out = append(out, FromGatewayService(s))
	}
	return out
}

// ── Routes ──────────────────────────────────────────────────────────────────

type GatewayRouteResponse struct {
	ID           string     `json:"id"`
	ServiceID    string     `json:"service_id"`
	ServiceName  string     `json:"service_name"`
	Name         string     `json:"name"`
	Methods      []string   `json:"methods"`
	Paths        []string   `json:"paths"`
	StripPath    bool       `json:"strip_path"`
	PreserveHost bool       `json:"preserve_host"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

func FromGatewayRoute(r domain.GatewayRoute) GatewayRouteResponse {
	return GatewayRouteResponse{
		ID: r.ID, ServiceID: r.ServiceID, ServiceName: r.ServiceName, Name: r.Name,
		Methods: nonNil(r.Methods), Paths: nonNil(r.Paths), StripPath: r.StripPath,
		PreserveHost: r.PreserveHost, Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func ToGatewayRouteList(list []domain.GatewayRoute) []GatewayRouteResponse {
	out := make([]GatewayRouteResponse, 0, len(list))
	for _, r := range list {
		out = append(out, FromGatewayRoute(r))
	}
	return out
}

// ── Consumers + keys ─────────────────────────────────────────────────────—

type GatewayConsumerResponse struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	CustomID  string     `json:"custom_id"`
	Tags      []string   `json:"tags"`
	Enabled   bool       `json:"enabled"`
	KeyCount  int        `json:"key_count"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func FromGatewayConsumer(c domain.GatewayConsumer) GatewayConsumerResponse {
	return GatewayConsumerResponse{
		ID: c.ID, Username: c.Username, CustomID: c.CustomID, Tags: nonNil(c.Tags),
		Enabled: c.Enabled, KeyCount: c.KeyCount, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func ToGatewayConsumerList(list []domain.GatewayConsumer) []GatewayConsumerResponse {
	out := make([]GatewayConsumerResponse, 0, len(list))
	for _, c := range list {
		out = append(out, FromGatewayConsumer(c))
	}
	return out
}

type GatewayKeyResponse struct {
	ID         string `json:"id"`
	ConsumerID string `json:"consumer_id"`
	Label      string `json:"label"`
	Prefix     string `json:"prefix"`
	// Plaintext нь зөвхөн үүсгэх хариунд буцна (бусад үед хоосон) — клиент үүнийг
	// нэг л удаа харж, хадгалах ёстой.
	Plaintext  string     `json:"plaintext,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
}

func FromGatewayKey(k domain.GatewayAPIKey) GatewayKeyResponse {
	return GatewayKeyResponse{
		ID: k.ID, ConsumerID: k.ConsumerID, Label: k.Label, Prefix: k.Prefix, Plaintext: k.Plaintext,
		LastUsedAt: k.LastUsedAt, ExpiresAt: k.ExpiresAt, Revoked: k.Revoked, CreatedAt: k.CreatedAt,
	}
}

func ToGatewayKeyList(list []domain.GatewayAPIKey) []GatewayKeyResponse {
	out := make([]GatewayKeyResponse, 0, len(list))
	for _, k := range list {
		out = append(out, FromGatewayKey(k))
	}
	return out
}

// ── Policies ─────────────────────────────────────────────────────────────—

type GatewayPolicyResponse struct {
	ID        string          `json:"id"`
	RouteID   *string         `json:"route_id"`
	RouteName string          `json:"route_name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt *time.Time      `json:"updated_at"`
}

func FromGatewayPolicy(p domain.GatewayPolicy) GatewayPolicyResponse {
	cfg := json.RawMessage(p.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	return GatewayPolicyResponse{
		ID: p.ID, RouteID: p.RouteID, RouteName: p.RouteName, Type: p.Type,
		Config: cfg, Enabled: p.Enabled, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func ToGatewayPolicyList(list []domain.GatewayPolicy) []GatewayPolicyResponse {
	out := make([]GatewayPolicyResponse, 0, len(list))
	for _, p := range list {
		out = append(out, FromGatewayPolicy(p))
	}
	return out
}

// ── Telemetry ────────────────────────────────────────────────────────────—

type GatewayRequestLogResponse struct {
	ID        string    `json:"id"`
	RouteName string    `json:"route_name"`
	Consumer  string    `json:"consumer"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	LatencyMS int       `json:"latency_ms"`
	ClientIP  string    `json:"client_ip"`
	CreatedAt time.Time `json:"created_at"`
}

func ToGatewayLogList(list []domain.GatewayRequestLog) []GatewayRequestLogResponse {
	out := make([]GatewayRequestLogResponse, 0, len(list))
	for _, l := range list {
		out = append(out, GatewayRequestLogResponse{
			ID: l.ID, RouteName: l.RouteName, Consumer: l.Consumer, Method: l.Method, Path: l.Path,
			Status: l.Status, LatencyMS: l.LatencyMS, ClientIP: l.ClientIP, CreatedAt: l.CreatedAt,
		})
	}
	return out
}

type GatewayOverviewResponse struct {
	Services       int                   `json:"services"`
	Routes         int                   `json:"routes"`
	Consumers      int                   `json:"consumers"`
	ActiveKeys     int                   `json:"active_keys"`
	Requests24h    int                   `json:"requests_24h"`
	Errors24h      int                   `json:"errors_24h"`
	RateLimited24h int                   `json:"rate_limited_24h"`
	ErrorRate      float64               `json:"error_rate"`
	AvgLatencyMS   int                   `json:"avg_latency_ms"`
	P95LatencyMS   int                   `json:"p95_latency_ms"`
	StatusBuckets  []GatewayStatusBucket `json:"status_buckets"`
	TopRoutes      []GatewayRouteStat    `json:"top_routes"`
}

type GatewayStatusBucket struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

type GatewayRouteStat struct {
	RouteName string `json:"route_name"`
	Count     int    `json:"count"`
}

func FromGatewayOverview(o domain.GatewayOverview) GatewayOverviewResponse {
	buckets := make([]GatewayStatusBucket, 0, len(o.StatusBuckets))
	for _, b := range o.StatusBuckets {
		buckets = append(buckets, GatewayStatusBucket{Class: b.Class, Count: b.Count})
	}
	top := make([]GatewayRouteStat, 0, len(o.TopRoutes))
	for _, t := range o.TopRoutes {
		top = append(top, GatewayRouteStat{RouteName: t.RouteName, Count: t.Count})
	}
	return GatewayOverviewResponse{
		Services: o.Services, Routes: o.Routes, Consumers: o.Consumers, ActiveKeys: o.ActiveKeys,
		Requests24h: o.Requests24h, Errors24h: o.Errors24h, RateLimited24h: o.RateLimited24h,
		ErrorRate: o.ErrorRate, AvgLatencyMS: o.AvgLatencyMS, P95LatencyMS: o.P95LatencyMS,
		StatusBuckets: buckets, TopRoutes: top,
	}
}

// nonNil нь nil slice-ийг хоосон slice болгож, JSON-д null биш [] болгоно.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
