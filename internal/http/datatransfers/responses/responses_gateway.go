// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
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

// ── Telemetry ────────────────────────────────────────────────────────────—

type GatewayRequestLogResponse struct {
	ID        string    `json:"id"`
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
			ID: l.ID, Method: l.Method, Path: l.Path,
			Status: l.Status, LatencyMS: l.LatencyMS, ClientIP: l.ClientIP, CreatedAt: l.CreatedAt,
		})
	}
	return out
}

type GatewayOverviewResponse struct {
	Services       int                   `json:"services"`
	Consumers      int                   `json:"consumers"`
	ActiveKeys     int                   `json:"active_keys"`
	Requests24h    int                   `json:"requests_24h"`
	Errors24h      int                   `json:"errors_24h"`
	RateLimited24h int                   `json:"rate_limited_24h"`
	ErrorRate      float64               `json:"error_rate"`
	AvgLatencyMS   int                   `json:"avg_latency_ms"`
	P95LatencyMS   int                   `json:"p95_latency_ms"`
	StatusBuckets  []GatewayStatusBucket `json:"status_buckets"`
	TopPaths       []GatewayPathStat     `json:"top_paths"`
}

type GatewayStatusBucket struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

type GatewayPathStat struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

func FromGatewayOverview(o domain.GatewayOverview) GatewayOverviewResponse {
	buckets := make([]GatewayStatusBucket, 0, len(o.StatusBuckets))
	for _, b := range o.StatusBuckets {
		buckets = append(buckets, GatewayStatusBucket{Class: b.Class, Count: b.Count})
	}
	top := make([]GatewayPathStat, 0, len(o.TopPaths))
	for _, t := range o.TopPaths {
		top = append(top, GatewayPathStat{Path: t.Path, Count: t.Count})
	}
	return GatewayOverviewResponse{
		Services: o.Services, Consumers: o.Consumers, ActiveKeys: o.ActiveKeys,
		Requests24h: o.Requests24h, Errors24h: o.Errors24h, RateLimited24h: o.RateLimited24h,
		ErrorRate: o.ErrorRate, AvgLatencyMS: o.AvgLatencyMS, P95LatencyMS: o.P95LatencyMS,
		StatusBuckets: buckets, TopPaths: top,
	}
}

// nonNil нь nil slice-ийг хоосон slice болгож, JSON-д null биш [] болгоно.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
