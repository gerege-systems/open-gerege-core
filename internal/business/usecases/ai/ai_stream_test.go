// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/pkg/gemini"
)

// fakeStreamer нь Generator + Streamer хоёуланг хэрэгжүүлнэ: өгсөн текстийг
// хэсэг хэсгээр нь (chunk) буцаана, эсвэл functionCall өгнө.
type fakeStreamer struct {
	rounds   [][]string             // дуудалт бүрийн текст хэсгүүд
	calls    []*gemini.FunctionCall // дуудалт бүрийн function call (nil = байхгүй)
	requests []gemini.Request
	round    int
}

func (f *fakeStreamer) GenerateContent(_ context.Context, req gemini.Request) (gemini.Response, error) {
	f.requests = append(f.requests, req)
	return gemini.Response{}, nil
}

func (f *fakeStreamer) StreamGenerateContent(_ context.Context, req gemini.Request, onText func(string) error) (gemini.Response, error) {
	f.requests = append(f.requests, req)
	i := f.round
	f.round++

	var parts []gemini.Part
	if i < len(f.rounds) {
		for _, chunk := range f.rounds[i] {
			parts = append(parts, gemini.Part{Text: chunk})
			if onText != nil {
				if err := onText(chunk); err != nil {
					return gemini.Response{}, err
				}
			}
		}
	}
	if i < len(f.calls) && f.calls[i] != nil {
		parts = append(parts, gemini.Part{FunctionCall: f.calls[i]})
	}
	return gemini.Response{Candidates: []gemini.Candidate{{Content: gemini.Content{Role: "model", Parts: parts}}}}, nil
}

func collect(t *testing.T, uc Usecase, req RunRequest) (events []StreamEvent, res RunResult) {
	t.Helper()
	res, err := uc.RunStream(context.Background(), req, func(ev StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	require.NoError(t, err)
	return events, res
}

func TestRunStream_EmitsDeltas(t *testing.T) {
	gen := &fakeStreamer{rounds: [][]string{{"Сайн ", "байна ", "уу"}}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	events, res := collect(t, uc, RunRequest{Prompt: "сайн уу"})

	var deltas []string
	for _, e := range events {
		if e.Delta != "" {
			deltas = append(deltas, e.Delta)
		}
	}
	assert.Equal(t, []string{"Сайн ", "байна ", "уу"}, deltas)
	assert.Equal(t, "Сайн байна уу", res.Reply)
	assert.False(t, res.Degraded)
}

// Дуут мессежийн хувьд эхний мөр нь хуулбар — тусдаа event болж, хариултын
// текстэд ОРОХГҮЙ (нэг дуудалтаар STT + хариулт).
func TestRunStream_ExtractsTranscript(t *testing.T) {
	gen := &fakeStreamer{rounds: [][]string{{"[ХЭЛСЭН] Хэрхэн нэвтр", "эх вэ?\nТа eID-", "ээр нэвтэрнэ."}}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	events, res := collect(t, uc, RunRequest{Audio: &Audio{Mime: "audio/webm", Data: "x"}})

	require.NotEmpty(t, events)
	assert.Equal(t, "Хэрхэн нэвтрэх вэ?", events[0].Transcript, "хуулбар эхний event")
	assert.Equal(t, "Хэрхэн нэвтрэх вэ?", res.Transcript)
	assert.Equal(t, "Та eID-ээр нэвтэрнэ.", res.Reply)
	assert.NotContains(t, res.Reply, transcriptMarker)

	// Audio байхад system prompt-д хуулбарын заавар + нэр томьёоны сануулга
	// (ойролцоо дуудлагатай үгийн андуурлыг багасгана) орсон байх ёстой.
	sys := gen.requests[0].SystemInstruction.Parts[0].Text
	assert.Contains(t, sys, transcriptMarker)
	assert.Contains(t, sys, "eID")
}

// Текст мессежид хуулбарын заавар нэмэгдэхгүй — илүү токен, илүү эрсдэл.
func TestRunStream_NoTranscriptInstructionForText(t *testing.T) {
	gen := &fakeStreamer{rounds: [][]string{{"за"}}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	_, _ = collect(t, uc, RunRequest{Prompt: "сайн уу"})
	assert.NotContains(t, gen.requests[0].SystemInstruction.Parts[0].Text, transcriptMarker)
}

// Model зааврыг дагаагүй (тэмдэггүй) бол бүх текст хариулт болно.
func TestRunStream_TranscriptMissingFallsBackToText(t *testing.T) {
	gen := &fakeStreamer{rounds: [][]string{{"Шууд хариулт байна.\nҮргэлжлэл."}}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	events, res := collect(t, uc, RunRequest{Audio: &Audio{Mime: "audio/webm", Data: "x"}})
	for _, e := range events {
		assert.Empty(t, e.Transcript)
	}
	assert.Equal(t, "Шууд хариулт байна.\nҮргэлжлэл.", res.Reply)
}

// Tool дуудлагын дараа хоёр дахь урсгал явна; өмнөх текст (байвал) хаягдана.
func TestRunStream_ToolRoundThenAnswer(t *testing.T) {
	repo := &fakeAIRepo{knowledge: nil}
	gen := &fakeStreamer{
		rounds: [][]string{{"Түр хүлээнэ үү."}, {"Мэдлэгийн сангаас олдсон хариу."}},
		calls:  []*gemini.FunctionCall{{Name: "search_knowledge", Args: map[string]any{"query": "eID"}}, nil},
	}
	uc := NewUsecase(gen, gen, repo, []ToolDef{KnowledgeSearchTool(repo, nil)}, Config{})

	events, res := collect(t, uc, RunRequest{Prompt: "eID гэж юу вэ?"})

	var sawReset bool
	for _, e := range events {
		if e.Reset {
			sawReset = true
		}
	}
	assert.True(t, sawReset, "tool дуудахын өмнөх текстийг хаяхыг заана")
	assert.Equal(t, "Мэдлэгийн сангаас олдсон хариу.", res.Reply)
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "search_knowledge", res.Steps[0].Tool)
}

// Streaming дэмждэггүй client (хуучин хуурамч Generator) — Run руу уналт.
func TestRunStream_FallsBackToNonStreaming(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("бүтэн хариулт")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	events, res := collect(t, uc, RunRequest{Prompt: "x"})
	require.Len(t, events, 1)
	assert.Equal(t, "бүтэн хариулт", events[0].Delta)
	assert.Equal(t, "бүтэн хариулт", res.Reply)
}

// Хариулт огт ирээгүй бол хэрэглэгчийн хэл дээрх fallback урсгалаар явна.
func TestRunStream_EmptyReplyFallsBack(t *testing.T) {
	gen := &fakeStreamer{rounds: [][]string{{}}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	events, res := collect(t, uc, RunRequest{Prompt: "x", Lang: "en"})
	require.NotEmpty(t, events)
	assert.True(t, res.Degraded)
	assert.NotEmpty(t, res.Reply)
	assert.True(t, strings.EqualFold(events[len(events)-1].Delta, res.Reply))
}
