//go:build integration

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// AI мэдлэгийн сангийн вектор хайлтын integration тест (жинхэнэ Postgres +
// pgvector): migration-ууд extension болон vector(768) баганыг үүсгэсэн эсэх,
// cosine эрэмбэлэлт зөв ажиллаж буй эсэх, backfill-ийн сонголт (embedding
// дутуу / hash зөрсөн) болон хадгалалт.
package ai_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/business/domain"
	aipg "template/internal/datasources/repositories/postgres/ai"
	"template/internal/test/testenv"
	"template/pkg/gemini"
)

// unitVector нь өгөгдсөн эхний хоёр хэмжээст утгатай, үлдсэн нь тэг нэгж
// вектор үүсгэнэ — cosine эрэмбийг таамаглахуйц болгоно.
func unitVector(x, y float32) []float32 {
	v := make([]float32, gemini.EmbedDim)
	norm := float32(math.Sqrt(float64(x*x + y*y)))
	v[0], v[1] = x/norm, y/norm
	return v
}

func TestKnowledgeVectorSearch(t *testing.T) {
	pool := testenv.StartPostgres(t)
	repo := aipg.NewAIRepository(pool)
	ctx := context.Background()

	// Migration 48-ийн корпус ачаалагдсан байх ёстой.
	pending, err := repo.ListKnowledgeForEmbedding(ctx, 200)
	require.NoError(t, err)
	require.NotEmpty(t, pending, "мэдлэгийн корпус seed хийгдсэн байх ёстой")

	// Бүх бичлэг embedding-гүй тул backfill жагсаалтад бүгд орно.
	byTitle := map[string]domain.AIKnowledgeChunk{}
	for _, c := range pending {
		byTitle[c.Title] = c
		assert.NotEmpty(t, c.Hash, "hash SQL талд бодогдоно")
		assert.Contains(t, c.Content, c.Title, "embed хийх текст гарчгийг агуулна")
	}

	// Хоёр бичлэгт хиймэл (мэдэгдэж буй чиглэлтэй) вектор өгнө.
	eid := byTitle["eID-ээр нэвтрэх"]
	gspace := byTitle["Gerege Space хадгалалт"]
	require.NotZero(t, eid.ID)
	require.NotZero(t, gspace.ID)
	eidID, gspaceID := eid.ID, gspace.ID

	// Backfill-ийн адилаар ЖИНХЭНЭ hash-тай нь хамт хадгална.
	require.NoError(t, repo.SaveKnowledgeEmbedding(ctx, eidID, unitVector(1, 0), eid.Hash))
	require.NoError(t, repo.SaveKnowledgeEmbedding(ctx, gspaceID, unitVector(0, 1), gspace.Hash))

	// (1,0) чиглэлд ойр асуулт → eID бичлэг тэргүүлнэ.
	hits, err := repo.SearchKnowledgeByVector(ctx, unitVector(0.9, 0.1), 5)
	require.NoError(t, err)
	require.NotEmpty(t, hits)
	assert.Equal(t, eidID, hits[0].ID)
	assert.Greater(t, hits[0].Score, 0.9, "cosine оноо 0..1 завсарт, ойр вектор өндөр")
	assert.NotEmpty(t, hits[0].Slug)
	assert.NotEmpty(t, hits[0].Source)

	// Эсрэг чиглэлд асуувал эрэмбэ солигдоно.
	hits, err = repo.SearchKnowledgeByVector(ctx, unitVector(0.1, 0.9), 5)
	require.NoError(t, err)
	require.NotEmpty(t, hits)
	assert.Equal(t, gspaceID, hits[0].ID)

	// Embedding хадгалсан бичлэгүүд backfill жагсаалтаас гарна.
	pending, err = repo.ListKnowledgeForEmbedding(ctx, 200)
	require.NoError(t, err)
	for _, c := range pending {
		assert.NotEqual(t, eidID, c.ID, "embedding-тэй бичлэг дахин embed хийгдэхгүй")
		assert.NotEqual(t, gspaceID, c.ID)
	}

	// Буруу (хуучирсан) hash-тай болговол дахин embed хийхээр эргэж орно.
	require.NoError(t, repo.SaveKnowledgeEmbedding(ctx, eidID, unitVector(1, 0), "stale-hash"))
	pending, err = repo.ListKnowledgeForEmbedding(ctx, 200)
	require.NoError(t, err)
	found := false
	for _, c := range pending {
		if c.ID == eidID {
			found = true
		}
	}
	assert.True(t, found, "агуулгын hash зөрсөн бичлэг дахин embed хийгдэнэ")
}

// Хоосон вектор нь query үүсгэлгүй хоосон үр дүн буцаана (дуудагч ILIKE рүү
// уналт хийнэ) — DB руу утгагүй хүсэлт явуулахгүй.
func TestKnowledgeVectorSearchEmptyVector(t *testing.T) {
	pool := testenv.StartPostgres(t)
	repo := aipg.NewAIRepository(pool)

	hits, err := repo.SearchKnowledgeByVector(context.Background(), nil, 5)
	require.NoError(t, err)
	assert.Empty(t, hits)
}
