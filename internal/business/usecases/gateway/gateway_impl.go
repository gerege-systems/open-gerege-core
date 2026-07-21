// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gateway

import (
	"context"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

type usecase struct {
	repo repointerface.GatewayRepository
}

func NewUsecase(repo repointerface.GatewayRepository) Usecase {
	return &usecase{repo: repo}
}

// ── Services ────────────────────────────────────────────────────────────────

func (u *usecase) ListServices(ctx context.Context) ([]domain.GatewayService, error) {
	return u.repo.ListServices(ctx)
}

func (u *usecase) CreateService(ctx context.Context, in ServiceInput) (domain.GatewayService, error) {
	svc, err := in.toDomain()
	if err != nil {
		return domain.GatewayService{}, err
	}
	return u.repo.CreateService(ctx, &svc)
}

func (u *usecase) UpdateService(ctx context.Context, id string, in ServiceInput) (domain.GatewayService, error) {
	svc, err := in.toDomain()
	if err != nil {
		return domain.GatewayService{}, err
	}
	svc.ID = id
	return u.repo.UpdateService(ctx, &svc)
}

func (u *usecase) DeleteService(ctx context.Context, id string) error {
	return u.repo.DeleteService(ctx, id)
}

func (in ServiceInput) toDomain() (domain.GatewayService, error) {
	name := strings.TrimSpace(in.Name)
	host := strings.TrimSpace(in.Host)
	if name == "" {
		return domain.GatewayService{}, apperror.BadRequest("service name is required")
	}
	if host == "" {
		return domain.GatewayService{}, apperror.BadRequest("service host is required")
	}
	protocol := strings.ToLower(strings.TrimSpace(in.Protocol))
	if protocol != "http" && protocol != "https" {
		protocol = "https"
	}
	port := in.Port
	if port <= 0 || port > 65535 {
		if protocol == "http" {
			port = 80
		} else {
			port = 443
		}
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		path = "/"
	}
	retries := in.Retries
	if retries < 0 {
		retries = 0
	}
	timeout := in.ConnectTimeout
	if timeout <= 0 {
		timeout = 60000
	}
	return domain.GatewayService{
		Name: name, Protocol: protocol, Host: host, Port: port, Path: path,
		Retries: retries, ConnectTimeout: timeout, Tags: cleanTags(in.Tags), Enabled: in.Enabled,
	}, nil
}

// ── Telemetry ────────────────────────────────────────────────────────────—

func (u *usecase) ListRequestLogs(ctx context.Context, limit int) ([]domain.GatewayRequestLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return u.repo.ListRequestLogs(ctx, limit)
}

func (u *usecase) Overview(ctx context.Context) (domain.GatewayOverview, error) {
	return u.repo.Overview(ctx)
}

// cleanTags нь хоосон/давхардсан tag-уудыг арилгаж, эрэмбэ хадгална.
func cleanTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// ── Request log (бодит хүсэлт) ─────────────────────────────────────────────—

// RecordRequest нь middleware-ээс ирсэн бодит /api хүсэлтийг лог-д бичнэ.
// Best-effort: DB алдааг залгина (лог бичилт хэрэглэгчийн хүсэлтийг блоклохгүй).
func (u *usecase) RecordRequest(ctx context.Context, in RequestLogInput) {
	_ = u.repo.CreateRequestLog(ctx, &domain.GatewayRequestLog{
		Method:    in.Method,
		Path:      in.Path,
		Status:    in.Status,
		LatencyMS: in.LatencyMS,
		ClientIP:  in.ClientIP,
	})
}
