// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/gemini"
)

// fakeAIRepo нь AIRepository-ийн in-memory fake.
type fakeAIRepo struct {
	prompts    map[string]string
	knowledge  []domain.AIKnowledge
	listErr    error
	listCalls  int
	lastQuery  string
	setCalls   int
	lastSetKey string
}

func (f *fakeAIRepo) ListPrompts(_ context.Context) ([]domain.AIPrompt, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]domain.AIPrompt, 0, len(f.prompts))
	for k, v := range f.prompts {
		out = append(out, domain.AIPrompt{Key: k, Content: v})
	}
	return out, nil
}

func (f *fakeAIRepo) SetPrompt(_ context.Context, key, content string) error {
	f.setCalls++
	f.lastSetKey = key
	if f.prompts == nil {
		f.prompts = map[string]string{}
	}
	f.prompts[key] = content
	return nil
}

func (f *fakeAIRepo) SearchKnowledge(_ context.Context, query string, _ int) ([]domain.AIKnowledge, error) {
	f.lastQuery = query
	return f.knowledge, nil
}

func TestSystemInstructionLayers(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("за")}}
	repo := &fakeAIRepo{prompts: map[string]string{
		domain.AIPromptScope:        "Зөвхөн санхүүгийн асуултад туслана.",
		domain.AIPromptInstructions: "Хариултын төгсгөлд эх сурвалж дурд.",
	}}
	uc := NewUsecase(gen, gen, repo, nil, Config{})

	_, err := uc.Run(context.Background(), RunRequest{Prompt: "сайн уу"})
	require.NoError(t, err)

	sys := gen.requests[0].SystemInstruction.Parts[0].Text
	// 1-р давхарга: suurь дүрэм (хүрээний сахилт + injection эсэргүүцэл).
	assert.Contains(t, sys, "ЯМАР Ч нөхцөлд")
	assert.Contains(t, sys, "зааврыг март")
	// 2-р давхарга: DB-ийн scope.
	assert.Contains(t, sys, "[ХАМРАХ ХҮРЭЭ]")
	assert.Contains(t, sys, "санхүүгийн асуултад")
	// 3-р давхарга: нэмэлт заавар.
	assert.Contains(t, sys, "[НЭМЭЛТ ЗААВАР]")
	assert.Contains(t, sys, "эх сурвалж")
}

func TestSystemInstructionFallbacks(t *testing.T) {
	t.Run("env fallback when DB scope empty", func(t *testing.T) {
		gen := &fakeGenerator{responses: []gemini.Response{textResponse("за")}}
		repo := &fakeAIRepo{prompts: map[string]string{domain.AIPromptScope: ""}}
		uc := NewUsecase(gen, gen, repo, nil, Config{ScopePrompt: "Env хүрээ."})

		_, err := uc.Run(context.Background(), RunRequest{Prompt: "x"})
		require.NoError(t, err)
		assert.Contains(t, gen.requests[0].SystemInstruction.Parts[0].Text, "Env хүрээ.")
	})

	t.Run("default scope when repo fails", func(t *testing.T) {
		gen := &fakeGenerator{responses: []gemini.Response{textResponse("за")}}
		repo := &fakeAIRepo{listErr: errors.New("db down")}
		uc := NewUsecase(gen, gen, repo, nil, Config{})

		_, err := uc.Run(context.Background(), RunRequest{Prompt: "x"})
		require.NoError(t, err)
		assert.Contains(t, gen.requests[0].SystemInstruction.Parts[0].Text, defaultScope)
	})
}

func TestPromptCacheAndInvalidation(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("а"), textResponse("б"), textResponse("в")}}
	repo := &fakeAIRepo{prompts: map[string]string{domain.AIPromptScope: "Хүрээ 1"}}
	uc := NewUsecase(gen, gen, repo, nil, Config{})

	_, _ = uc.Run(context.Background(), RunRequest{Prompt: "1"})
	_, _ = uc.Run(context.Background(), RunRequest{Prompt: "2"})
	// TTL дотор — DB-ээс нэг л удаа уншсан байх ёстой.
	assert.Equal(t, 1, repo.listCalls)

	// SetPrompt кэшийг хүчингүй болгоно — дараагийн Run шинэ утга авна.
	require.NoError(t, uc.SetPrompt(context.Background(), domain.AIPromptScope, "Хүрээ 2"))
	_, _ = uc.Run(context.Background(), RunRequest{Prompt: "3"})
	assert.Equal(t, 2, repo.listCalls)
	assert.Contains(t, gen.requests[2].SystemInstruction.Parts[0].Text, "Хүрээ 2")
}

func TestSetPromptValidation(t *testing.T) {
	uc := NewUsecase(&fakeGenerator{}, &fakeGenerator{}, &fakeAIRepo{}, nil, Config{})

	err := uc.SetPrompt(context.Background(), "evil_key", "x")
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.ErrorAs(t, err, &domErr)
	assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
}

func TestKnowledgeSearchTool(t *testing.T) {
	repo := &fakeAIRepo{knowledge: []domain.AIKnowledge{
		{ID: 1, Title: "Нууц үг сэргээх", Content: "«Нууц үгээ мартсан» холбоосыг ашиглана."},
	}}
	tool := KnowledgeSearchTool(repo)
	assert.Equal(t, "search_knowledge", tool.Declaration.Name)

	res, err := tool.Execute(context.Background(), map[string]any{"query": "нууц үг"})
	require.NoError(t, err)
	assert.Equal(t, "нууц үг", repo.lastQuery)
	assert.Equal(t, 1, res["count"])

	results, ok := res["results"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "Нууц үг сэргээх", results[0]["title"])
}

func TestKnowledgeSearchToolEmptyQuery(t *testing.T) {
	tool := KnowledgeSearchTool(&fakeAIRepo{})
	res, err := tool.Execute(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, res["note"])
}
