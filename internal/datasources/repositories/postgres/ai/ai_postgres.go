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
	"template/pkg/gemini"

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
	// ILIKE нь ЗӨВХӨН fallback: семантик (вектор) хайлт боломжгүй үед
	// (Gemini түлхүүргүй, эсвэл embedding хараахан backfill хийгдээгүй)
	// түлхүүр үгээр ядаж ямар нэг зүйл олохын тулд үлдээв.
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, content, tags, COALESCE(slug, ''), COALESCE(source, '')
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
		if scanErr := rows.Scan(&k.ID, &k.Title, &k.Content, &k.Tags, &k.Slug, &k.Source); scanErr != nil {
			return nil, fmt.Errorf("scan ai knowledge: %w", scanErr)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// SearchKnowledgeByVector нь cosine зайгаар (pgvector <=> оператор) хамгийн
// ойр бичлэгүүдийг буцаана. Score = 1 - зай (1 бол ижил утга).
//
// pgx нь extension-ий vector төрлийг мэддэггүй тул векторыг текст литералаар
// дамжуулж ::vector-оор cast хийнэ — планд нөлөөлөхгүй, HNSW индекс ажиллана.
func (r *aiRepository) SearchKnowledgeByVector(ctx context.Context, embedding []float32, limit int) ([]domain.AIKnowledge, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	lit := gemini.VectorLiteral(embedding)
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, content, tags, COALESCE(slug, ''), COALESCE(source, ''),
		       1 - (embedding <=> $1::vector) AS score
		  FROM ai_knowledge
		 WHERE embedding IS NOT NULL
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`, lit, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search ai knowledge: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AIKnowledge, 0, limit)
	for rows.Next() {
		var k domain.AIKnowledge
		if scanErr := rows.Scan(&k.ID, &k.Title, &k.Content, &k.Tags, &k.Slug, &k.Source, &k.Score); scanErr != nil {
			return nil, fmt.Errorf("scan ai knowledge vector row: %w", scanErr)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListKnowledgeForEmbedding нь embedding дутуу, эсвэл агуулга нь өөрчлөгдсөн
// (хадгалсан content_hash одоогийн hash-тай зөрсөн) бичлэгүүдийг буцаана.
// Hash-ийг SQL талд бодох нь Go тал давхар уншиж тооцоолохоос хямд.
func (r *aiRepository) ListKnowledgeForEmbedding(ctx context.Context, limit int) ([]domain.AIKnowledgeChunk, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, title || E'\n\n' || content AS body,
		       encode(sha256((title || E'\n\n' || content)::bytea), 'hex') AS hash
		  FROM ai_knowledge
		 WHERE embedding IS NULL
		    OR content_hash IS DISTINCT FROM
		       encode(sha256((title || E'\n\n' || content)::bytea), 'hex')
		 ORDER BY id
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai knowledge for embedding: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AIKnowledgeChunk, 0, limit)
	for rows.Next() {
		var c domain.AIKnowledgeChunk
		if scanErr := rows.Scan(&c.ID, &c.Title, &c.Content, &c.Hash); scanErr != nil {
			return nil, fmt.Errorf("scan ai knowledge chunk: %w", scanErr)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveKnowledgeEmbedding нь векторыг hash-тай нь хамт хадгална. app_user-д
// зөвхөн эдгээр багана дээр UPDATE эрх өгсөн (migration 47) — агуулга нь
// migration/seed-ээр удирдагдсан хэвээр.
func (r *aiRepository) SaveKnowledgeEmbedding(ctx context.Context, id int, embedding []float32, hash string) error {
	if len(embedding) == 0 {
		return fmt.Errorf("save ai knowledge embedding: empty vector")
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_knowledge
		   SET embedding = $2::vector, content_hash = $3, embedded_at = now()
		 WHERE id = $1`, id, gemini.VectorLiteral(embedding), hash)
	if err != nil {
		return fmt.Errorf("save ai knowledge embedding: %w", err)
	}
	return nil
}
