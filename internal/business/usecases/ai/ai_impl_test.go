// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/pkg/gemini"
)

// fakeGenerator нь дуудлага бүрт дараалсан хариу буцаадаг Generator fake.
type fakeGenerator struct {
	responses []gemini.Response
	errs      []error
	calls     int
	requests  []gemini.Request
}

func (f *fakeGenerator) GenerateContent(_ context.Context, req gemini.Request) (gemini.Response, error) {
	i := f.calls
	f.calls++
	f.requests = append(f.requests, req)
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	var resp gemini.Response
	if i < len(f.responses) {
		resp = f.responses[i]
	}
	return resp, err
}

func textResponse(text string) gemini.Response {
	return gemini.Response{Candidates: []gemini.Candidate{{
		Content: gemini.Content{Role: "model", Parts: []gemini.Part{{Text: text}}},
	}}}
}

func functionCallResponse(name string, args map[string]any) gemini.Response {
	return gemini.Response{Candidates: []gemini.Candidate{{
		Content: gemini.Content{Role: "model", Parts: []gemini.Part{{
			FunctionCall: &gemini.FunctionCall{Name: name, Args: args},
		}}},
	}}}
}

func echoTool(name string) ToolDef {
	return ToolDef{
		Declaration: gemini.FunctionDeclaration{Name: name, Description: "echo"},
		Execute: func(_ context.Context, args map[string]any) (map[string]any, error) {
			return map[string]any{"echo": args}, nil
		},
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name         string
		gen          *fakeGenerator
		tools        []ToolDef
		req          RunRequest
		wantReply    string
		wantDegraded bool
		wantSteps    int
		wantErr      bool
	}{
		{
			name:      "plain text reply",
			gen:       &fakeGenerator{responses: []gemini.Response{textResponse("Сайн байна уу!")}},
			req:       RunRequest{Prompt: "сайн уу"},
			wantReply: "Сайн байна уу!",
		},
		{
			name: "function call then final answer",
			gen: &fakeGenerator{responses: []gemini.Response{
				functionCallResponse("echo_tool", map[string]any{"x": "1"}),
				textResponse("Үр дүн: 1"),
			}},
			tools:     []ToolDef{echoTool("echo_tool")},
			req:       RunRequest{Prompt: "x-ийг хэл"},
			wantReply: "Үр дүн: 1",
			wantSteps: 1,
		},
		{
			name: "unknown tool reported to model, pipeline continues",
			gen: &fakeGenerator{responses: []gemini.Response{
				functionCallResponse("no_such_tool", nil),
				textResponse("Уучлаарай, тэр мэдээлэл алга."),
			}},
			tools:     []ToolDef{echoTool("echo_tool")},
			req:       RunRequest{Prompt: "?"},
			wantReply: "Уучлаарай, тэр мэдээлэл алга.",
			wantSteps: 1,
		},
		{
			name:         "transient gemini failure returns fallback",
			gen:          &fakeGenerator{errs: []error{errors.New("gemini: 3 attempts failed")}},
			req:          RunRequest{Prompt: "сайн уу"},
			wantReply:    fallbackReply("mn"),
			wantDegraded: true,
		},
		{
			name:    "not configured returns internal error",
			gen:     &fakeGenerator{errs: []error{gemini.ErrNotConfigured}},
			req:     RunRequest{Prompt: "сайн уу"},
			wantErr: true,
		},
		{
			name: "max steps reached returns fallback",
			gen: &fakeGenerator{responses: []gemini.Response{
				functionCallResponse("echo_tool", nil),
				functionCallResponse("echo_tool", nil),
			}},
			tools:        []ToolDef{echoTool("echo_tool")},
			req:          RunRequest{Prompt: "loop"},
			wantReply:    fallbackReply("mn"),
			wantDegraded: true,
			wantSteps:    2,
		},
		{
			name:         "empty model reply returns fallback",
			gen:          &fakeGenerator{responses: []gemini.Response{textResponse("")}},
			req:          RunRequest{Prompt: "сайн уу"},
			wantReply:    fallbackReply("mn"),
			wantDegraded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{MaxSteps: 2}
			uc := NewUsecase(tt.gen, tt.gen, nil, tt.tools, cfg)

			res, err := uc.Run(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				var domErr *apperror.DomainError
				require.ErrorAs(t, err, &domErr)
				assert.Equal(t, apperror.ErrTypeInternal, domErr.Type)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantReply, res.Reply)
			assert.Equal(t, tt.wantDegraded, res.Degraded)
			assert.Len(t, res.Steps, tt.wantSteps)
		})
	}
}

func TestRunSendsSystemInstructionAndHistory(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("за")}}
	uc := NewUsecase(gen, gen, nil, DefaultTools(), Config{})

	_, err := uc.Run(context.Background(), RunRequest{
		Prompt: "одоо хэдэн цаг болж байна?",
		History: []Turn{
			{Role: "user", Text: "сайн уу"},
			{Role: "model", Text: "Сайн байна уу!"},
		},
	})
	require.NoError(t, err)
	require.Len(t, gen.requests, 1)

	req := gen.requests[0]
	require.NotNil(t, req.SystemInstruction)
	// Хэл заагаагүй бол өгөгдмөл (mn) хэлний дүрэм орно.
	assert.Contains(t, req.SystemInstruction.Parts[0].Text, uiLangNames[DefaultLang])
	// history 2 + шинэ prompt 1
	require.Len(t, req.Contents, 3)
	assert.Equal(t, "user", req.Contents[0].Role)
	assert.Equal(t, "model", req.Contents[1].Role)
	assert.Equal(t, "одоо хэдэн цаг болж байна?", req.Contents[2].Parts[0].Text)
	// tools зарлагдсан байх ёстой
	require.Len(t, req.Tools, 1)
	assert.Equal(t, "get_server_time", req.Tools[0].FunctionDeclarations[0].Name)
}

func TestRunTruncatesLongHistory(t *testing.T) {
	gen := &fakeGenerator{responses: []gemini.Response{textResponse("за")}}
	uc := NewUsecase(gen, gen, nil, nil, Config{})

	history := make([]Turn, 30)
	for i := range history {
		history[i] = Turn{Role: "user", Text: "x"}
	}
	_, err := uc.Run(context.Background(), RunRequest{Prompt: "y", History: history})
	require.NoError(t, err)
	// maxHistoryTurns(20) + шинэ prompt 1
	assert.Len(t, gen.requests[0].Contents, maxHistoryTurns+1)
}

func TestServerTimeTool(t *testing.T) {
	tool := serverTimeTool()
	res, err := tool.Execute(context.Background(), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, res["datetime"])
	assert.Equal(t, "Asia/Ulaanbaatar", res["timezone"])
}

// Хэрэглэгчийн хэл нь system prompt болон degraded мессежид тусна.
func TestRun_UsesRequestLanguage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		lang     string
		wantLang string
	}{
		{name: "chinese", lang: "zh", wantLang: "zh"},
		{name: "russian", lang: "ru", wantLang: "ru"},
		{name: "english", lang: "en", wantLang: "en"},
		{name: "unknown falls back to default", lang: "xx", wantLang: DefaultLang},
		{name: "empty falls back to default", lang: "", wantLang: DefaultLang},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gen := &fakeGenerator{responses: []gemini.Response{textResponse("ok")}}
			uc := NewUsecase(gen, gen, nil, nil, Config{})

			_, err := uc.Run(context.Background(), RunRequest{Prompt: "hi", Lang: tt.lang})
			require.NoError(t, err)
			require.Len(t, gen.requests, 1)

			prompt := gen.requests[0].SystemInstruction.Parts[0].Text
			assert.Contains(t, prompt, uiLangNames[tt.wantLang])
			// Зорилтот хэл дээрх заавар prompt-ын ТӨГСГӨЛД байх ёстой.
			assert.True(t, strings.HasSuffix(prompt, langDirectives[tt.wantLang]),
				"prompt нь %q-ээр төгсөх ёстой", langDirectives[tt.wantLang])
			// Зөвхөн нэг хэлний дүрэм орсон байх ёстой — өөр хэлний нэр ч,
			// өөр хэлний заавар ч prompt-д огт байхгүй.
			for code, name := range uiLangNames {
				if code != tt.wantLang {
					assert.NotContains(t, prompt, name)
					assert.NotContains(t, prompt, langDirectives[code])
				}
			}
			// Заавар нь эхний дүрэмд болон төгсгөлд — нийт хоёр удаа давтагдана
			// (primacy + recency): system prompt монголоор бичигдсэн тул
			// зорилтот хэл дээрх заавар л model-ыг барина.
			assert.Equal(t, 2, strings.Count(prompt, langDirectives[tt.wantLang]))
		})
	}
}

// Gemini унасан үед degraded мессеж нь хэрэглэгчийн хэл дээр байна.
func TestRun_FallbackIsLocalised(t *testing.T) {
	for _, lang := range []string{"mn", "en", "zh", "ru"} {
		t.Run(lang, func(t *testing.T) {
			gen := &fakeGenerator{errs: []error{errors.New("gemini: 3 attempts failed")}}
			uc := NewUsecase(gen, gen, nil, nil, Config{})

			res, err := uc.Run(context.Background(), RunRequest{Prompt: "hi", Lang: lang})
			require.NoError(t, err)
			assert.True(t, res.Degraded)
			assert.Equal(t, fallbackReplies[lang], res.Reply)
		})
	}
}
