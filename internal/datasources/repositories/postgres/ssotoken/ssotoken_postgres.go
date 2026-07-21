// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ssotoken нь иргэний dgov-SSO OAuth токенуудыг sso_tokens хүснэгтэд
// AES-GCM-ээр шифрлэж хадгална (SSO eID proxy-д зориулж). users_postgres.go-ийн
// адил withRLS транзакцаар ажиллана: SSO callback дээр RoleService, eID унших/
// refresh дээр RoleUser (өөрийн мөр). Багана зөвхөн шифр текст агуулна.
package ssotoken

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/business/domain"
	"template/internal/datasources/rls"
	"template/pkg/crypto"
)

type ssoTokenRepository struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

// NewSSOTokenRepository нь sso_tokens repo үүсгэнэ. cipher нь INTEGRATION_ENC_KEY-
// ээс гарсан AES-GCM шифрлэгч (token-ыг storage-д шифрлэнэ).
func NewSSOTokenRepository(pool *pgxpool.Pool, cipher *crypto.Cipher) *ssoTokenRepository {
	return &ssoTokenRepository{pool: pool, cipher: cipher}
}

// withRLS нь ssouser_postgres.go-ийн загварын дагуу app.user_id / app.user_role
// GUC-уудыг нэг транзакцийн туршид тавьж fn-г ажиллуулна.
func (r *ssoTokenRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit returns ErrTxClosed — expected

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

// Upsert нь токенуудыг шифрлэж хадгална (user_id-ээр дарж бичнэ).
func (r *ssoTokenRepository) Upsert(ctx context.Context, userID string, tok domain.SSOToken) error {
	accessEnc, err := r.cipher.Encrypt(tok.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	refreshEnc, err := r.cipher.Encrypt(tok.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx, `
			INSERT INTO sso_tokens (user_id, access_token_enc, refresh_token_enc, access_expires_at, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (user_id) DO UPDATE SET
				access_token_enc  = EXCLUDED.access_token_enc,
				refresh_token_enc = EXCLUDED.refresh_token_enc,
				access_expires_at = EXCLUDED.access_expires_at,
				updated_at        = now()
		`, userID, accessEnc, refreshEnc, tok.AccessExpiresAt)
		return execErr
	})
}

// Get нь хадгалагдсан токенуудыг тайлж буцаана; мөр байхгүй бол
// domain.ErrSSOTokenNotFound.
func (r *ssoTokenRepository) Get(ctx context.Context, userID string) (domain.SSOToken, error) {
	var accessEnc, refreshEnc string
	var expiresAt time.Time
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT access_token_enc, refresh_token_enc, access_expires_at FROM sso_tokens WHERE user_id = $1`,
			userID,
		).Scan(&accessEnc, &refreshEnc, &expiresAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SSOToken{}, domain.ErrSSOTokenNotFound
	}
	if err != nil {
		return domain.SSOToken{}, err
	}
	access, err := r.cipher.Decrypt(accessEnc)
	if err != nil {
		return domain.SSOToken{}, fmt.Errorf("decrypt access token: %w", err)
	}
	refresh, err := r.cipher.Decrypt(refreshEnc)
	if err != nil {
		return domain.SSOToken{}, fmt.Errorf("decrypt refresh token: %w", err)
	}
	return domain.SSOToken{AccessToken: access, RefreshToken: refresh, AccessExpiresAt: expiresAt}, nil
}
