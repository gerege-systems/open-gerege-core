// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package language нь languages / translations хүснэгтийн Postgres gateway юм.
// Хэрэглэгч-тус-бүрийн биш нийтийн config тул Row-Level Security-д хамаарахгүй
// (plain pool query) — themes-тэй ижил зарчим.
package language

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/core/business/domain"
	repointerface "github.com/gerege-systems/open-gerege-core/core/datasources/repositories/interface"
)

type languageRepository struct {
	pool *pgxpool.Pool
}

func NewLanguageRepository(pool *pgxpool.Pool) repointerface.LanguageRepository {
	return &languageRepository{pool: pool}
}

const languageCols = `code, label, locale, enabled, is_builtin, sort_order, created_at, updated_at`

// pgUniqueViolation нь Postgres-ийн давхардсан түлхүүрийн кодыг илэрхийлнэ.
const pgUniqueViolation = "23505"

func (r *languageRepository) ListLanguages(ctx context.Context) ([]domain.Language, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+languageCols+` FROM languages ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("list languages: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Language])
	if err != nil {
		return nil, fmt.Errorf("scan languages: %w", err)
	}
	return list, nil
}

func (r *languageRepository) ListEnabledLanguages(ctx context.Context) ([]domain.Language, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+languageCols+` FROM languages WHERE enabled ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("list enabled languages: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Language])
	if err != nil {
		return nil, fmt.Errorf("scan enabled languages: %w", err)
	}
	return list, nil
}

func (r *languageRepository) GetLanguage(ctx context.Context, code string) (domain.Language, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+languageCols+` FROM languages WHERE code = $1`, code)
	if err != nil {
		return domain.Language{}, fmt.Errorf("query language: %w", err)
	}
	lang, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Language])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Language{}, apperror.NotFound("language not found")
		}
		return domain.Language{}, fmt.Errorf("scan language: %w", err)
	}
	return lang, nil
}

func (r *languageRepository) CreateLanguage(ctx context.Context, lang domain.Language) (domain.Language, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO languages (code, label, locale, enabled, is_builtin, sort_order)
		 VALUES ($1, $2, $3, $4, false, $5)
		 RETURNING `+languageCols,
		lang.Code, lang.Label, lang.Locale, lang.Enabled, lang.SortOrder)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.Language{}, apperror.Conflict("language already exists")
		}
		return domain.Language{}, fmt.Errorf("insert language: %w", err)
	}
	created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Language])
	if err != nil {
		return domain.Language{}, fmt.Errorf("scan created language: %w", err)
	}
	return created, nil
}

// UpdateLanguage нь COALESCE-оор хэсэгчилсэн шинэчлэлт хийнэ — nil параметр нь
// одоогийн утгыг хэвээр үлдээнэ.
func (r *languageRepository) UpdateLanguage(ctx context.Context, code string, patch domain.LanguagePatch) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE languages SET
		     label      = COALESCE($2, label),
		     locale     = COALESCE($3, locale),
		     enabled    = COALESCE($4, enabled),
		     sort_order = COALESCE($5, sort_order),
		     updated_at = now()
		 WHERE code = $1`,
		code, patch.Label, patch.Locale, patch.Enabled, patch.SortOrder)
	if err != nil {
		return fmt.Errorf("update language: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("language not found")
	}
	return nil
}

func (r *languageRepository) DeleteLanguage(ctx context.Context, code string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM languages WHERE code = $1`, code)
	if err != nil {
		return fmt.Errorf("delete language: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("language not found")
	}
	return nil
}

func (r *languageRepository) GetDictionary(ctx context.Context, code string) (map[string]string, error) {
	return r.collectPairs(ctx,
		`SELECT key, value FROM translations WHERE lang_code = $1`, code, "dictionary")
}

func (r *languageRepository) GetTranslationSources(ctx context.Context, code string) (map[string]string, error) {
	return r.collectPairs(ctx,
		`SELECT key, source FROM translations WHERE lang_code = $1`, code, "translation sources")
}

// collectPairs нь хоёр баганат (текст, текст) үр дүнг газрын зураг болгоно.
func (r *languageRepository) collectPairs(ctx context.Context, sql, code, what string) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, sql, code)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", what, err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan %s: %w", what, err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", what, err)
	}
	return out, nil
}

// UpsertTranslations нь бүх мөрийг НЭГ хүсэлтээр бичнэ (unnest). Мянган түлхүүрт
// мөр тус бүрийн round-trip хийвэл удаан бөгөөд AI орчуулгын дараах бичилт
// хэдэн зуун мөртэй байдаг.
func (r *languageRepository) UpsertTranslations(ctx context.Context, code string, entries map[string]string, source string) error {
	if len(entries) == 0 {
		return nil
	}
	keys := make([]string, 0, len(entries))
	values := make([]string, 0, len(entries))
	for k, v := range entries {
		keys = append(keys, k)
		values = append(values, v)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO translations (lang_code, key, value, source, updated_at)
		 SELECT $1, k, v, $4, now()
		 FROM unnest($2::text[], $3::text[]) AS t(k, v)
		 ON CONFLICT (lang_code, key) DO UPDATE
		 SET value = EXCLUDED.value, source = EXCLUDED.source, updated_at = now()`,
		code, keys, values, source)
	if err != nil {
		return fmt.Errorf("upsert translations: %w", err)
	}
	return nil
}
