// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package userintegrations нь user_integrations хүснэгтийн Postgres gateway.
// Хэрэглэгч-тус-бүрийн мэдрэмтгий өгөгдөл тул query бүр withRLS транзакцаар
// (app.user_id / app.user_role session хувьсагчдыг тогтоож) ажиллана.
package userintegrations

import (
	"context"
	"fmt"

	"template/internal/business/domain"
	"template/internal/datasources/records"
	repointerface "template/internal/datasources/repositories/interface"
	"template/internal/datasources/rls"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewUserIntegrationsRepository(pool *pgxpool.Pool) repointerface.UserIntegrationRepository {
	return &repository{pool: pool}
}

// withRLS нь query-г транзакцид боож, context-оос (rls.FromContext) уншсан
// identity-гээр RLS session хувьсагчдыг (SET LOCAL дүйцэлтэй) тогтооно. Identity
// байхгүй бол хоосон болж бодлогууд бүх мөрийг хаана (fail-closed).
func (r *repository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit-ийн дараа rollback no-op; алдааны замд буцах нь хамаагүй

	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.user_id',$1,true), set_config('app.user_role',$2,true)`,
		id.UserID, string(id.Role),
	); err != nil {
		return fmt.Errorf("set rls session context: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *repository) Upsert(ctx context.Context, in *domain.UserIntegration) (domain.UserIntegration, error) {
	const (
		repositoryName = "userintegrations"
		funcName       = "Upsert"
		fileName       = "userintegrations_postgres.go"
	)
	var refresh *string
	if in.RefreshToken != "" {
		refresh = &in.RefreshToken
	}

	var stored records.UserIntegrations
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`INSERT INTO user_integrations (user_id, provider, access_token, refresh_token, expires_at)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (user_id, provider) DO UPDATE
			   SET access_token = EXCLUDED.access_token,
			       refresh_token = EXCLUDED.refresh_token,
			       expires_at = EXCLUDED.expires_at,
			       updated_at = now()
			 RETURNING `+records.UserIntegrationsColumns,
			in.UserID, in.Provider, in.AccessToken, refresh, in.ExpiresAt)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.UserIntegrations])
		return scanErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to upsert user integration", logger.Fields{
			"repository": repositoryName, "method": funcName, "file": fileName,
			"error": err.Error(), "user_id": in.UserID, "provider": in.Provider,
		})
		return domain.UserIntegration{}, err
	}
	return stored.ToV1Domain(), nil
}

func (r *repository) ListByUser(ctx context.Context, userID string) ([]domain.UserIntegration, error) {
	const (
		repositoryName = "userintegrations"
		funcName       = "ListByUser"
		fileName       = "userintegrations_postgres.go"
	)
	var stored []records.UserIntegrations
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.UserIntegrationsColumns+`
			 FROM user_integrations WHERE user_id = $1 ORDER BY created_at`, userID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectRows(rows, pgx.RowToStructByName[records.UserIntegrations])
		return scanErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to list user integrations", logger.Fields{
			"repository": repositoryName, "method": funcName, "file": fileName,
			"error": err.Error(), "user_id": userID,
		})
		return nil, err
	}
	out := make([]domain.UserIntegration, 0, len(stored))
	for _, s := range stored {
		out = append(out, s.ToV1Domain())
	}
	return out, nil
}

func (r *repository) DeleteByUserAndProvider(ctx context.Context, userID, provider string) error {
	const (
		repositoryName = "userintegrations"
		funcName       = "DeleteByUserAndProvider"
		fileName       = "userintegrations_postgres.go"
	)
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx,
			`DELETE FROM user_integrations WHERE user_id = $1 AND provider = $2`, userID, provider)
		return execErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to delete user integration", logger.Fields{
			"repository": repositoryName, "method": funcName, "file": fileName,
			"error": err.Error(), "user_id": userID, "provider": provider,
		})
		return err
	}
	return nil
}
