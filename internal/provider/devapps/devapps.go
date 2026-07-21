// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package devapps нь developer.dgov.mn-ээр үүсгэсэн OAuth2 апп (RP)-уудын
// байнгын бүртгэл (PostgreSQL). Мөр бүр нь client_id-г эзэмшигч иргэний
// eid_sub-тай холбоно — OAuth2 client бүртгэлд "owner" гэдэг ойлголт байхгүй тул
// энэ мөр нь эрх мэдлийн эх сурвалж (устгахдаа client-тэй хамт устгана). sso-dgov-mn-
// ий internal/devapps-аас шилжүүлэв.
package devapps

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	// ClientID нь OAuth2 client_id (мөн developer-т харагдах public апп таних).
	// Формат: "app-<16-hex>".
	ClientID string

	// OwnerEIDSub нь аппыг үүсгэсэн иргэний eid_sub. Энэ мөрийн бүх өөрчлөлт
	// `sub` нь таарах access_token шаардана.
	OwnerEIDSub string

	Name         string
	RedirectURIs []string
	Scopes       []string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

// New нь өгөгдсөн pool дээрх devapps store-ыг буцаана (schema migrate хийгдсэн байх).
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const appCols = `client_id, owner_eid_sub, name, redirect_uris, scopes, created_at, updated_at`

func scanApp(row pgx.Row) (*App, error) {
	var a App
	if err := row.Scan(&a.ClientID, &a.OwnerEIDSub, &a.Name, &a.RedirectURIs, &a.Scopes, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// Create нь шинэ апп мөр оруулна. Дуудагч эхлээд харгалзах OAuth2 client-ыг
// үүсгэсэн байх ёстой; энэ бичилт амжилтгүй бол client-ыг буцаана.
func (s *Store) Create(ctx context.Context, a App) (*App, error) {
	if a.RedirectURIs == nil {
		a.RedirectURIs = []string{}
	}
	if a.Scopes == nil {
		a.Scopes = []string{}
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO developer_apps (client_id, owner_eid_sub, name, redirect_uris, scopes)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+appCols,
		a.ClientID, a.OwnerEIDSub, a.Name, a.RedirectURIs, a.Scopes)
	created, err := scanApp(row)
	if isUniqueViolation(err) {
		return nil, ErrAlreadyExists
	}
	return created, err
}

// Get нь ClientID-аар аппыг буцаана. Өөрчлөхөөс өмнө OwnerMatches ашиглаж
// эзэмшлийг шалга; энэ метод дангаараа auth хийхгүй.
func (s *Store) Get(ctx context.Context, clientID string) (*App, bool) {
	row := s.pool.QueryRow(ctx, `SELECT `+appCols+` FROM developer_apps WHERE client_id=$1`, clientID)
	a, err := scanApp(row)
	if err != nil {
		return nil, false
	}
	return a, true
}

// ListAll нь бүх аппыг шинэ эхэнд буцаана. Зөвхөн admin гадаргуу.
func (s *Store) ListAll(ctx context.Context) []App {
	return s.query(ctx, `SELECT `+appCols+` FROM developer_apps ORDER BY created_at DESC`)
}

// ListByOwner нь sub-ийн эзэмшдэг бүх аппыг шинэ эхэнд буцаана.
func (s *Store) ListByOwner(ctx context.Context, sub string) []App {
	return s.query(ctx, `SELECT `+appCols+` FROM developer_apps WHERE owner_eid_sub=$1 ORDER BY created_at DESC`, sub)
}

func (s *Store) query(ctx context.Context, sql string, args ...any) []App {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]App, 0, 8)
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return out
		}
		out = append(out, *a)
	}
	return out
}

// Update нь Name + RedirectURIs-г өөрчилнө. Дуудагч эзэмшлийг шалгасан байх ёстой.
// ClientID, OwnerEIDSub нь өөрчлөгдөшгүй. Хоосон name / nil uris хэвээр үлдэнэ.
func (s *Store) Update(ctx context.Context, clientID, name string, redirectURIs []string) (*App, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE developer_apps SET
			name          = COALESCE(NULLIF($2,''), name),
			redirect_uris = COALESCE($3, redirect_uris),
			updated_at    = now()
		WHERE client_id = $1
		RETURNING `+appCols,
		clientID, name, redirectURIs)
	a, err := scanApp(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *Store) Delete(ctx context.Context, clientID string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM developer_apps WHERE client_id=$1`, clientID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// OwnerMatches нь апп байгаа БА OwnerEIDSub нь sub-тай тэнцүү эсэхийг буцаана.
func (s *Store) OwnerMatches(ctx context.Context, clientID, sub string) bool {
	a, ok := s.Get(ctx, clientID)
	return ok && a.OwnerEIDSub == sub
}

var (
	ErrNotFound      = errors.New("devapps: app not found")
	ErrAlreadyExists = errors.New("devapps: client_id already exists")
)

// isUniqueViolation нь Postgres unique-constraint алдаа (SQLSTATE 23505) эсэхийг
// мэдээлнэ.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}
