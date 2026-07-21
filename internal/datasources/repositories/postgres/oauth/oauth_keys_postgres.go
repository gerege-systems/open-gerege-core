// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
)

// keyRepository нь id_token гарын үсгийн түлхүүрүүдийг хадгална. oauth_clients-
// ийн адил системийн тохиргоо тул RLS-гүй; хувийн түлхүүр нь мөрөндөө
// шифрлэгдсэн байдлаар хамгаалагдана.
type keyRepository struct {
	pool *pgxpool.Pool
}

func NewKeyRepository(pool *pgxpool.Pool) *keyRepository {
	return &keyRepository{pool: pool}
}

const keyColumns = ` kid, alg, private_key_enc, public_jwk, active, created_at, retired_at`

func scanKey(row pgx.Row) (domain.SigningKey, error) {
	var k domain.SigningKey
	err := row.Scan(&k.KID, &k.Alg, &k.PrivateKeyEnc, &k.PublicJWK, &k.Active, &k.CreatedAt, &k.RetiredAt)
	return k, err
}

// Active нь гарын үсэг зурахад ашиглах цорын ганц идэвхтэй түлхүүрийг буцаана.
// Байхгүй бол apperror.NotFound (дуудагч шинийг үүсгэнэ).
func (r *keyRepository) Active(ctx context.Context) (domain.SigningKey, error) {
	k, err := scanKey(r.pool.QueryRow(ctx,
		`SELECT`+keyColumns+` FROM oauth_signing_keys WHERE active LIMIT 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SigningKey{}, apperror.NotFound("no active signing key")
	}
	if err != nil {
		return domain.SigningKey{}, fmt.Errorf("get active signing key: %w", err)
	}
	return k, nil
}

// All нь JWKS-д нийтлэх бүх түлхүүрийг буцаана — retire хийсэн нь ч ОРНО,
// эс бөгөөс тэдгээрээр гарын үсэг зурсан, хараахан хүчинтэй id_token-ууд
// шалгагдахаа болино.
func (r *keyRepository) All(ctx context.Context) ([]domain.SigningKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT`+keyColumns+` FROM oauth_signing_keys ORDER BY active DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list signing keys: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SigningKey, 0, 2)
	for rows.Next() {
		k, scanErr := scanKey(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan signing key: %w", scanErr)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Insert нь шинэ түлхүүр нэмнэ. active=true-тэй нэмэхээс өмнө дуудагч хуучныг
// Retire хийсэн байх ёстой (нэг идэвхтэй түлхүүрийн unique index хамгаална).
func (r *keyRepository) Insert(ctx context.Context, k domain.SigningKey) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO oauth_signing_keys (kid, alg, private_key_enc, public_jwk, active)
		VALUES ($1, $2, $3, $4, $5)`,
		k.KID, k.Alg, k.PrivateKeyEnc, k.PublicJWK, k.Active)
	if err != nil {
		return fmt.Errorf("insert signing key: %w", err)
	}
	return nil
}

// RetireActive нь одоогийн идэвхтэй түлхүүрийг тэтгэвэрт гаргана (JWKS-д үлдэнэ).
func (r *keyRepository) RetireActive(ctx context.Context) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE oauth_signing_keys SET active = false, retired_at = now() WHERE active`)
	if err != nil {
		return fmt.Errorf("retire signing key: %w", err)
	}
	return nil
}
