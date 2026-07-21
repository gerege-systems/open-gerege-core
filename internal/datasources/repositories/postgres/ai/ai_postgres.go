// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package ai нь ai_prompts (тохируулдаг prompt давхаргууд) болон ai_knowledge
// (AI-ийн хайдаг мэдлэгийн сан) хүснэгтүүдийн Postgres gateway юм. Хоёулаа
// хэрэглэгч-тус-бүрийн биш лавлах өгөгдөл тул Row-Level Security-д
// хамаарахгүй (plain pool query).
package ai

import (
	"context"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"

	"github.com/jackc/pgx/v5/pgxpool"
)

type aiRepository struct {
	pool *pgxpool.Pool
}

func NewAIRepository(pool *pgxpool.Pool) repointerface.AIRepository {
	return &aiRepository{pool: pool}
}

func (r *aiRepository) ListPrompts(ctx context.Context) ([]domain.AIPrompt, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, content, updated_at FROM ai_prompts ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list ai prompts: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AIPrompt, 0, 2)
	for rows.Next() {
		var p domain.AIPrompt
		if scanErr := rows.Scan(&p.Key, &p.Content, &p.UpdatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan ai prompt: %w", scanErr)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetPrompt нь зөвхөн UPDATE хийдэг — зөвшөөрөгдсөн key-үүд migration-д seed
// хийгдсэн тул дурын шинэ давхарга нэмэгдэхгүй (prompt гадаргууг хаалттай
// байлгана).
func (r *aiRepository) SetPrompt(ctx context.Context, key, content string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE ai_prompts SET content = $2, updated_at = now() WHERE key = $1`, key, content)
	if err != nil {
		return fmt.Errorf("set ai prompt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("prompt not found")
	}
	return nil
}

func (r *aiRepository) SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.AIKnowledge, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	// ILIKE — template хэмжээнд хангалттай; том сан дээр энэ query-г
	// tsvector (full-text) эсвэл pgvector (semantic) хайлтаар солино.
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, content, tags
		  FROM ai_knowledge
		 WHERE title ILIKE '%' || $1 || '%'
		    OR content ILIKE '%' || $1 || '%'
		    OR $1 = ANY(tags)
		 ORDER BY updated_at DESC NULLS LAST, id DESC
		 LIMIT $2`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search ai knowledge: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AIKnowledge, 0, limit)
	for rows.Next() {
		var k domain.AIKnowledge
		if scanErr := rows.Scan(&k.ID, &k.Title, &k.Content, &k.Tags); scanErr != nil {
			return nil, fmt.Errorf("scan ai knowledge: %w", scanErr)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
