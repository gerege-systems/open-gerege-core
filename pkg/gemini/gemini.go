// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gemini нь Google Gemini API-ийн хөнгөн REST client юм — SDK
// ашиглахгүйгээр generateContent endpoint-ийг шууд дууддаг. Function
// calling-ийг бүрэн дэмжинэ: AI ямар функц дуудахаа шийдэж, backend
// (usecases/ai) гүйцэтгэдэг. Түр зуурын алдаан дээр (429/5xx/сүлжээ)
// exponential backoff-той 3 удаа дахин оролддог.
//
//	POST {base}/models/{model}:generateContent
//	Auth: x-goog-api-key: <GEMINI_API_KEY>
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotConfigured нь GEMINI_API_KEY тохируулагдаагүй үед буцдаг sentinel —
// template нь Gemini-гүйгээр boot хийгдэх боломжтой хэвээр (verify.Client-тэй
// ижил загвар).
var ErrNotConfigured = errors.New("gemini: API key not configured (GEMINI_API_KEY)")

const (
	defaultBase  = "https://generativelanguage.googleapis.com/v1beta"
	defaultModel = "gemini-2.5-flash"
	maxRespBytes = 4 << 20 // Gemini-ийн урт хариу + candidates-ийг багтаана.

	// maxAttempts = 1 анхны оролдлого + 2 дахин оролдлого. Backoff нь
	// initialBackoff * 2^attempt (500ms → 1s).
	maxAttempts    = 3
	initialBackoff = 500 * time.Millisecond
)

// --- Gemini REST wire төрлүүд (зөвхөн ашигладаг талбарууд) ---

// Part нь Content доторх нэг хэсэг — текст, function дуудлага (model-ээс),
// function-ий үр дүн (backend-ээс буцааж өгдөг), эсвэл inline media (audio
// оролт / TTS гаралт) гэсэн төрлүүдийн нэг.
type Part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
	InlineData       *Blob             `json:"inlineData,omitempty"`
}

// Blob нь inline media — Data нь base64 кодлогдсон байт, MimeType нь
// "audio/webm" гэх мэт төрөл. Audio ойлголтод оролт, TTS-д гаралт болж явна.
type Blob struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// FunctionCall нь model-ийн "энэ функцийг эдгээр аргументаар дууд" гэсэн шийдвэр.
type FunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// FunctionResponse нь backend дээр гүйцэтгэсэн функцийн үр дүнг model руу
// буцааж өгөх хэлбэр.
type FunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// Content нь нэг ээлжийн (turn) агуулга. Role: "user" | "model".
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

// FunctionDeclaration нь model-д зарлах функцийн тодорхойлолт. Parameters нь
// OpenAPI/JSON Schema хэлбэртэй map (хатуу төрөл шаардахгүй — дуудагч өөрөө
// зарладаг).
type FunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// Tool нь function calling-д зарлагдах функцуудын багц.
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations"`
}

// GenerationConfig нь generation-ий сонголттой тохиргоо. ResponseModalities
// + SpeechConfig нь TTS model-уудад ("AUDIO" modality) хэрэглэгдэнэ.
type GenerationConfig struct {
	Temperature        *float64      `json:"temperature,omitempty"`
	MaxOutputTokens    int           `json:"maxOutputTokens,omitempty"`
	ResponseModalities []string      `json:"responseModalities,omitempty"`
	SpeechConfig       *SpeechConfig `json:"speechConfig,omitempty"`
}

// SpeechConfig нь TTS дуу хоолойн сонголт.
type SpeechConfig struct {
	VoiceConfig *VoiceConfig `json:"voiceConfig,omitempty"`
}

type VoiceConfig struct {
	PrebuiltVoiceConfig *PrebuiltVoiceConfig `json:"prebuiltVoiceConfig,omitempty"`
}

type PrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName"`
}

// Request нь generateContent хүсэлтийн body.
type Request struct {
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	Contents          []Content         `json:"contents"`
	Tools             []Tool            `json:"tools,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
}

// Candidate нь model-ийн нэг хариулт.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
}

// Response нь generateContent хариу.
type Response struct {
	Candidates []Candidate `json:"candidates"`
}

// Text нь эхний candidate-ийн бүх текст хэсгийг нэгтгэж буцаана.
func (r Response) Text() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return strings.TrimSpace(b.String())
}

// FunctionCalls нь эхний candidate-ийн бүх function дуудлагыг буцаана —
// хоосон бол model текстээр хариулсан гэсэн үг.
func (r Response) FunctionCalls() []FunctionCall {
	if len(r.Candidates) == 0 {
		return nil
	}
	var calls []FunctionCall
	for _, p := range r.Candidates[0].Content.Parts {
		if p.FunctionCall != nil {
			calls = append(calls, *p.FunctionCall)
		}
	}
	return calls
}

// InlineAudio нь эхний candidate-ийн эхний audio inlineData-г буцаана (TTS
// гаралт) — байхгүй бол nil.
func (r Response) InlineAudio() *Blob {
	if len(r.Candidates) == 0 {
		return nil
	}
	for _, p := range r.Candidates[0].Content.Parts {
		if p.InlineData != nil && strings.HasPrefix(p.InlineData.MimeType, "audio/") {
			return p.InlineData
		}
	}
	return nil
}

// ModelContent нь эхний candidate-ийн Content-ийг буцаана — function calling
// давталтад model-ийн ээлжийг conversation руу буцааж нэмэхэд хэрэглэнэ.
func (r Response) ModelContent() Content {
	if len(r.Candidates) == 0 {
		return Content{Role: "model"}
	}
	c := r.Candidates[0].Content
	if c.Role == "" {
		c.Role = "model"
	}
	return c
}

// Generator нь Gemini дуудлагын хийсвэрлэл — тестэд хуурамчаар тавихад хялбар.
type Generator interface {
	GenerateContent(ctx context.Context, req Request) (Response, error)
}

// Client нь Gemini API руу залгадаг HTTP client.
type Client struct {
	base   string
	apiKey string
	model  string
	http   *http.Client
	// sleep-ийг тестэд override хийнэ (бодит backoff хүлээхгүйн тулд).
	sleep func(ctx context.Context, d time.Duration) error
}

// NewClient нь Gemini client үүсгэнэ. base/model хоосон бол өгөгдмөл утга
// авна. apiKey хоосон бол GenerateContent нь ErrNotConfigured буцаана.
func NewClient(base, apiKey, model string) *Client {
	if base == "" {
		base = defaultBase
	}
	if model == "" {
		model = defaultModel
	}
	return &Client{
		base:   strings.TrimRight(base, "/"),
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 60 * time.Second},
		sleep:  sleepCtx,
	}
}

// GenerateContent нь generateContent-ийг дуудаж, түр зуурын алдаан дээр
// (сүлжээ / 429 / 5xx) exponential backoff-той дахин оролдоно. Бүх оролдлого
// амжилтгүй бол сүүлчийн алдааг буцаана — fallback мессежийг дуудагч
// (usecase) шийднэ.
func (c *Client) GenerateContent(ctx context.Context, req Request) (Response, error) {
	if c.apiKey == "" {
		return Response{}, ErrNotConfigured
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff << (attempt - 1)
			if err := c.sleep(ctx, backoff); err != nil {
				return Response{}, fmt.Errorf("gemini: retry wait: %w", err)
			}
		}

		resp, retryable, err := c.generateOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryable {
			return Response{}, err
		}
	}
	return Response{}, fmt.Errorf("gemini: %d attempts failed: %w", maxAttempts, lastErr)
}

// generateOnce нь нэг HTTP оролдлого хийнэ. retryable нь алдааг дахин
// оролдох утгатай эсэхийг (сүлжээ / 429 / 5xx) илэрхийлнэ.
func (c *Client) generateOnce(ctx context.Context, req Request) (Response, bool, error) {
	buf, err := json.Marshal(req)
	if err != nil {
		return Response{}, false, fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", c.base, c.model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return Response{}, false, fmt.Errorf("gemini: build request: %w", err)
	}
	httpReq.Header.Set("x-goog-api-key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.http.Do(httpReq)
	if err != nil {
		// Контекст цуцлагдсан бол дахин оролдоод нэмэргүй.
		if ctx.Err() != nil {
			return Response{}, false, fmt.Errorf("gemini: http: %w", err)
		}
		return Response{}, true, fmt.Errorf("gemini: http: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(httpResp.Body, maxRespBytes))

	switch {
	case httpResp.StatusCode == http.StatusTooManyRequests || httpResp.StatusCode >= 500:
		return Response{}, true, fmt.Errorf("gemini: status %d: %s", httpResp.StatusCode, snippet(raw))
	case httpResp.StatusCode >= 300:
		// Бусад 4xx (буруу хүсэлт, эрхгүй түлхүүр) — дахин оролдоод нэмэргүй.
		return Response{}, false, fmt.Errorf("gemini: status %d: %s", httpResp.StatusCode, snippet(raw))
	}

	var out Response
	if jErr := json.Unmarshal(raw, &out); jErr != nil {
		return Response{}, false, fmt.Errorf("gemini: decode response: %w", jErr)
	}
	return out, false, nil
}

// sleepCtx нь context-г хүндэтгэдэг sleep.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// snippet нь алдааны body-г log-д аюулгүй хэмжээнд тайрна.
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
