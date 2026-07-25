// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/business/domain"
	"template/pkg/gemini"
)

// fakeEmbedder нь gemini.Embedder-ийн хуурамч хэрэгжүүлэлт. vector нь бүх
// текстэд ижил вектор буцаана (агуулга нь тестэд хамаагүй).
type fakeEmbedder struct {
	calls     int
	lastTexts []string
	lastTask  string
	err       error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string, taskType string) ([][]float32, error) {
	f.calls++
	f.lastTexts = texts
	f.lastTask = taskType
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float32, 0, len(texts))
	for range texts {
		out = append(out, []float32{0.1, 0.2, 0.3})
	}
	return out, nil
}

// --- search_knowledge: вектор хайлт ба уналт ---

func TestKnowledgeSearch_UsesVectorFirst(t *testing.T) {
	repo := &fakeAIRepo{
		vectorHits: []domain.AIKnowledge{
			{ID: 7, Title: "eID нэвтрэлт", Content: "QR / РД push", Source: "docs", Score: 0.82},
		},
		knowledge: []domain.AIKnowledge{{ID: 99, Title: "ILIKE-ээр олдсон"}},
	}
	emb := &fakeEmbedder{}

	res, err := KnowledgeSearchTool(repo, emb).Execute(context.Background(),
		map[string]any{"query": "яаж нэвтрэх вэ"})

	require.NoError(t, err)
	assert.Equal(t, "vector", res["mode"])
	assert.Equal(t, 1, res["count"])
	// Асуултыг RETRIEVAL_QUERY төрлөөр embed хийсэн байх ёстой.
	assert.Equal(t, gemini.TaskQuery, emb.lastTask)
	assert.Equal(t, []string{"яаж нэвтрэх вэ"}, emb.lastTexts)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, repo.lastVector)
	// Түлхүүр үгийн хайлт огт хийгдээгүй байх ёстой.
	assert.Empty(t, repo.lastQuery)

	results := res["results"].([]map[string]any)
	assert.Equal(t, "eID нэвтрэлт", results[0]["title"])
	assert.Equal(t, "docs", results[0]["source"])
	assert.Equal(t, 0.82, results[0]["score"])
}

func TestKnowledgeSearch_FallsBackToKeyword(t *testing.T) {
	lowScore := []domain.AIKnowledge{{ID: 1, Title: "Хамааралгүй", Score: 0.2}}

	tests := []struct {
		name string
		repo *fakeAIRepo
		emb  gemini.Embedder
	}{
		{
			name: "embedder байхгүй",
			repo: &fakeAIRepo{knowledge: []domain.AIKnowledge{{ID: 2, Title: "Түлхүүр үг"}}},
			emb:  nil,
		},
		{
			name: "embedding алдаа",
			repo: &fakeAIRepo{knowledge: []domain.AIKnowledge{{ID: 2, Title: "Түлхүүр үг"}}},
			emb:  &fakeEmbedder{err: gemini.ErrNotConfigured},
		},
		{
			name: "вектор хайлт унасан",
			repo: &fakeAIRepo{vectorErr: errors.New("db down"), knowledge: []domain.AIKnowledge{{ID: 2, Title: "Түлхүүр үг"}}},
			emb:  &fakeEmbedder{},
		},
		{
			name: "таарц босгоос доогуур",
			repo: &fakeAIRepo{vectorHits: lowScore, knowledge: []domain.AIKnowledge{{ID: 2, Title: "Түлхүүр үг"}}},
			emb:  &fakeEmbedder{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := KnowledgeSearchTool(tt.repo, tt.emb).Execute(context.Background(),
				map[string]any{"query": "нэвтрэлт"})

			require.NoError(t, err)
			assert.Equal(t, "keyword", res["mode"])
			assert.Equal(t, "нэвтрэлт", tt.repo.lastQuery, "ILIKE хайлт хийгдсэн байх ёстой")
			results := res["results"].([]map[string]any)
			require.Len(t, results, 1)
			assert.Equal(t, "Түлхүүр үг", results[0]["title"])
		})
	}
}

// --- backfill ---

func TestEmbedKnowledge_SavesVectorsWithHash(t *testing.T) {
	repo := &fakeAIRepo{pending: []domain.AIKnowledgeChunk{
		{ID: 1, Title: "нэг", Content: "нэг\n\nагуулга", Hash: "h1"},
		{ID: 2, Title: "хоёр", Content: "хоёр\n\nагуулга", Hash: "h2"},
	}}
	emb := &fakeEmbedder{}
	uc := NewUsecase(&fakeGenerator{}, &fakeGenerator{}, repo, nil, Config{Embedder: emb})

	n, err := uc.EmbedKnowledge(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, gemini.TaskDocument, emb.lastTask, "бичлэгийг RETRIEVAL_DOCUMENT-ээр embed хийнэ")
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, repo.saved[1])
	assert.Equal(t, "h2", repo.savedHash[2])
}

func TestEmbedKnowledge_NoEmbedderIsNoop(t *testing.T) {
	repo := &fakeAIRepo{pending: []domain.AIKnowledgeChunk{{ID: 1, Content: "x", Hash: "h"}}}
	uc := NewUsecase(&fakeGenerator{}, &fakeGenerator{}, repo, nil, Config{})

	n, err := uc.EmbedKnowledge(context.Background())

	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, repo.saved)
}

func TestEmbedKnowledge_NotConfiguredStopsQuietly(t *testing.T) {
	repo := &fakeAIRepo{pending: []domain.AIKnowledgeChunk{{ID: 1, Content: "x", Hash: "h"}}}
	uc := NewUsecase(&fakeGenerator{}, &fakeGenerator{}, repo,
		nil, Config{Embedder: &fakeEmbedder{err: gemini.ErrNotConfigured}})

	n, err := uc.EmbedKnowledge(context.Background())

	require.NoError(t, err, "түлхүүр байхгүй нь алдаа биш")
	assert.Zero(t, n)
}

func TestEmbedKnowledge_PropagatesRealError(t *testing.T) {
	repo := &fakeAIRepo{pending: []domain.AIKnowledgeChunk{{ID: 1, Content: "x", Hash: "h"}}}
	uc := NewUsecase(&fakeGenerator{}, &fakeGenerator{}, repo,
		nil, Config{Embedder: &fakeEmbedder{err: errors.New("gemini down")}})

	_, err := uc.EmbedKnowledge(context.Background())
	require.Error(t, err)
}
