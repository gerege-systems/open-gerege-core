// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package oauth нь өөрийн OAuth2/OIDC provider-ийн Postgres gateway. Энэ файл нь
// client (relying party) бүртгэлийг хариуцна.
//
// oauth_clients нь системийн тохиргооны хүснэгт (хэрэглэгч-тус-бүрийн БИШ) тул
// `applications` / `gateway_services`-ийн адил RLS-гүй, plain pool query-ээр
// ажиллана — зөвшөөрлийг route давхарга (gateway.manage) шийднэ.
package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
)

type clientRepository struct {
	pool *pgxpool.Pool
}

// NewClientRepository нь oauth_clients дээрх gateway-г буцаана.
func NewClientRepository(pool *pgxpool.Pool) *clientRepository {
	return &clientRepository{pool: pool}
}

const clientColumns = `
	client_id, client_name, secret_hash, token_endpoint_auth_method, app_type,
	grant_types, response_types, scopes, redirect_uris, post_logout_redirect_uris,
	tags, enabled, created_by, created_at, updated_at`

func scanClient(row pgx.Row) (domain.OAuthClient, error) {
	var c domain.OAuthClient
	err := row.Scan(
		&c.ClientID, &c.ClientName, &c.SecretHash, &c.TokenEndpointAuthMethod, &c.AppType,
		&c.GrantTypes, &c.ResponseTypes, &c.Scopes, &c.RedirectURIs, &c.PostLogoutRedirectURIs,
		&c.Tags, &c.Enabled, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// List нь бүх client-ыг шинэ→хуучин дарааллаар буцаана.
func (r *clientRepository) List(ctx context.Context) ([]domain.OAuthClient, error) {
	rows, err := r.pool.Query(ctx, `SELECT`+clientColumns+` FROM oauth_clients ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query oauth clients: %w", err)
	}
	defer rows.Close()

	out := make([]domain.OAuthClient, 0)
	for rows.Next() {
		c, scanErr := scanClient(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan oauth client: %w", scanErr)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get нь нэг client-ыг client_id-гээр буцаана. Олдоогүй бол apperror.NotFound.
func (r *clientRepository) Get(ctx context.Context, clientID string) (domain.OAuthClient, error) {
	c, err := scanClient(r.pool.QueryRow(ctx,
		`SELECT`+clientColumns+` FROM oauth_clients WHERE client_id = $1`, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthClient{}, apperror.NotFound("application not found")
	}
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("get oauth client: %w", err)
	}
	return c, nil
}

// Create нь шинэ client бичнэ. client_id давхардвал apperror.Conflict.
func (r *clientRepository) Create(ctx context.Context, c domain.OAuthClient) (domain.OAuthClient, error) {
	out, err := scanClient(r.pool.QueryRow(ctx, `
		INSERT INTO oauth_clients (
			client_id, client_name, secret_hash, token_endpoint_auth_method, app_type,
			grant_types, response_types, scopes, redirect_uris, post_logout_redirect_uris,
			tags, enabled, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING`+clientColumns,
		c.ClientID, c.ClientName, c.SecretHash, c.TokenEndpointAuthMethod, c.AppType,
		strList(c.GrantTypes), strList(c.ResponseTypes), strList(c.Scopes),
		strList(c.RedirectURIs), strList(c.PostLogoutRedirectURIs),
		strList(c.Tags), c.Enabled, c.CreatedBy,
	))
	if isUniqueViolation(err) {
		return domain.OAuthClient{}, apperror.Conflict("client_id already exists")
	}
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("insert oauth client: %w", err)
	}
	return out, nil
}

// Update нь client-ын тохиргоог шинэчилнэ. secret_hash-д ХҮРЭХГҮЙ — түүнийг
// зөвхөн SetSecretHash сольж чадна (санамсаргүй secret-ийг устгахаас сэргийлнэ).
func (r *clientRepository) Update(ctx context.Context, c domain.OAuthClient) (domain.OAuthClient, error) {
	out, err := scanClient(r.pool.QueryRow(ctx, `
		UPDATE oauth_clients SET
			client_name = $2, token_endpoint_auth_method = $3, app_type = $4,
			grant_types = $5, response_types = $6, scopes = $7,
			redirect_uris = $8, post_logout_redirect_uris = $9,
			tags = $10, enabled = $11, updated_at = now()
		WHERE client_id = $1
		RETURNING`+clientColumns,
		c.ClientID, c.ClientName, c.TokenEndpointAuthMethod, c.AppType,
		strList(c.GrantTypes), strList(c.ResponseTypes), strList(c.Scopes),
		strList(c.RedirectURIs), strList(c.PostLogoutRedirectURIs),
		strList(c.Tags), c.Enabled,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthClient{}, apperror.NotFound("application not found")
	}
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("update oauth client: %w", err)
	}
	return out, nil
}

// SetSecretHash нь client secret-ийн hash-ыг сольно (rotate / гараар оноох).
func (r *clientRepository) SetSecretHash(ctx context.Context, clientID, hash string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE oauth_clients SET secret_hash = $2, updated_at = now() WHERE client_id = $1`,
		clientID, hash)
	if err != nil {
		return fmt.Errorf("update oauth client secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("application not found")
	}
	return nil
}

// Delete нь client-ыг устгана. Түүний code/token/consent-ууд FK cascade-ээр устана.
func (r *clientRepository) Delete(ctx context.Context, clientID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM oauth_clients WHERE client_id = $1`, clientID)
	if err != nil {
		return fmt.Errorf("delete oauth client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("application not found")
	}
	return nil
}

// strList нь nil slice-ийг Postgres-ийн хоосон text[] болгоно (NOT NULL багана).
func strList(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
