// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

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
)

// EmbedDim нь text-embedding-004 моделийн гаралтын хэмжээ. Өгөгдлийн сангийн
// vector(768) баганатай ЗААВАЛ таарна — өөр хэмжээтэй model руу шилжвэл
// migration-аар баганаа солих шаардлагатай.
const EmbedDim = 768

// embedModelFallbacks нь оролдох embedding model-ууд (эрэмбээр). Google нь
// хуучин model-уудыг тухайн API хувилбар/түлхүүрийн хувьд хаадаг тул нэг
// нэрэнд бүү найд: 404 (model олдсонгүй / embedContent дэмжихгүй) гарвал
// дараагийнх руу шилжинэ. Ажилласан нэрийг тогтоож аваад дараагийн
// дуудалтуудад шууд хэрэглэнэ.
var embedModelFallbacks = []string{
	"gemini-embedding-001",
	"text-embedding-004",
	"embedding-001",
}

// defaultEmbedModel нь эхний оролдлогын model (тохиргоо хоосон үед).
const defaultEmbedModel = "gemini-embedding-001"

// errEmbedModelUnavailable нь тухайн model энэ түлхүүр/хувилбарт байхгүйг
// заана — дараагийн нэр рүү шилжих дохио (хэрэглэгчид харагдахгүй).
var errEmbedModelUnavailable = errors.New("gemini: embedding model unavailable")

// Embedding-ийн task type-ууд. Retrieval-д баримт болон асуултыг өөр өөр
// төрлөөр embed хийхэд таарц мэдэгдэхүйц сайжирдаг (Google-ийн зөвлөмж).
const (
	TaskDocument = "RETRIEVAL_DOCUMENT"
	TaskQuery    = "RETRIEVAL_QUERY"
)

// ErrEmbedShape нь хариу хүлээгдсэн хэлбэртэй ирээгүйг заана (текстийн тоо
// таарахгүй, эсвэл вектор хоосон/буруу хэмжээтэй).
var ErrEmbedShape = errors.New("gemini: unexpected embedding response shape")

// Embedder нь текстүүдийг вектор болгодог гадаргуу. Тестэд хуурамч
// хэрэгжүүлэлт өгөх боломжтой (Generator-той ижил загвар).
type Embedder interface {
	// Embed нь текст бүрийн хувьд EmbedDim урттай вектор буцаана.
	// taskType нь TaskDocument (мэдлэгийн сангийн бичлэг) эсвэл TaskQuery
	// (хэрэглэгчийн асуулт).
	Embed(ctx context.Context, texts []string, taskType string) ([][]float32, error)
}

// --- wire төрлүүд ---

type embedContent struct {
	Parts []Part `json:"parts"`
}

type embedRequest struct {
	Model                string       `json:"model"`
	Content              embedContent `json:"content"`
	TaskType             string       `json:"taskType,omitempty"`
	OutputDimensionality int          `json:"outputDimensionality,omitempty"`
}

type batchEmbedRequest struct {
	Requests []embedRequest `json:"requests"`
}

type embedValues struct {
	Values []float32 `json:"values"`
}

type batchEmbedResponse struct {
	Embeddings []embedValues `json:"embeddings"`
}

// Embed нь batchEmbedContents-ийг дуудаж, түр зуурын алдаан дээр
// GenerateContent-тэй ижил exponential backoff-оор дахин оролдоно.
// API key байхгүй бол ErrNotConfigured — дуудагч (backfill / хайлт) үүнийг
// хараад түлхүүр үгийн хайлт руу уналт хийнэ.
func (c *Client) Embed(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, ErrNotConfigured
	}
	if len(texts) == 0 {
		return nil, nil
	}

	var lastErr error
	for _, model := range c.embedCandidates() {
		out, err := c.embedWithModel(ctx, model, texts, taskType)
		if err == nil {
			c.rememberEmbedModel(model)
			return out, nil
		}
		lastErr = err
		if !errors.Is(err, errEmbedModelUnavailable) {
			return nil, err
		}
		// 404 — энэ нэр тухайн түлхүүрт байхгүй; дараагийнхыг оролдоно.
	}
	return nil, lastErr
}

// embedCandidates нь оролдох model-уудын жагсаалтыг эрэмбэлж буцаана:
// нэгэнт ажилласан нь тогтоогдсон бол зөвхөн түүнийг, эс бөгөөс
// тохируулсан нэр + fallback-ууд (давхардалгүй).
func (c *Client) embedCandidates() []string {
	c.embedMu.Lock()
	resolved := c.embedResolved
	configured := c.embedModel
	c.embedMu.Unlock()

	if resolved != "" {
		return []string{resolved}
	}
	out := make([]string, 0, len(embedModelFallbacks)+1)
	seen := map[string]bool{}
	for _, m := range append([]string{configured}, embedModelFallbacks...) {
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func (c *Client) rememberEmbedModel(model string) {
	c.embedMu.Lock()
	c.embedResolved = model
	c.embedMu.Unlock()
}

// embedWithModel нь нэг model дээр retry/backoff-той оролдоно.
func (c *Client) embedWithModel(ctx context.Context, model string, texts []string, taskType string) ([][]float32, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff << (attempt - 1)
			if err := c.sleep(ctx, backoff); err != nil {
				return nil, fmt.Errorf("gemini: retry wait: %w", err)
			}
		}

		out, retryable, err := c.embedOnce(ctx, model, texts, taskType)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, fmt.Errorf("gemini: %d attempts failed: %w", maxAttempts, lastErr)
}

func (c *Client) embedOnce(ctx context.Context, model string, texts []string, taskType string) (vectors [][]float32, retryable bool, err error) {
	if model == "" {
		model = defaultEmbedModel
	}
	qualified := "models/" + model

	body := batchEmbedRequest{Requests: make([]embedRequest, 0, len(texts))}
	for _, t := range texts {
		body.Requests = append(body.Requests, embedRequest{
			Model:    qualified,
			Content:  embedContent{Parts: []Part{{Text: t}}},
			TaskType: taskType,
			// Model-ууд өөр өөр хэмжээтэй (gemini-embedding-001 нь өгөгдмөл
			// 3072) тул DB-ийн vector(768)-д таарахаар шууд заана.
			OutputDimensionality: EmbedDim,
		})
	}

	buf, mErr := json.Marshal(body)
	if mErr != nil {
		return nil, false, fmt.Errorf("gemini: marshal embed request: %w", mErr)
	}

	url := fmt.Sprintf("%s/%s:batchEmbedContents", c.base, qualified)
	httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if reqErr != nil {
		return nil, false, fmt.Errorf("gemini: build embed request: %w", reqErr)
	}
	httpReq.Header.Set("x-goog-api-key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, doErr := c.http.Do(httpReq)
	if doErr != nil {
		if ctx.Err() != nil {
			return nil, false, fmt.Errorf("gemini: embed http: %w", doErr)
		}
		return nil, true, fmt.Errorf("gemini: embed http: %w", doErr)
	}
	defer func() { _ = httpResp.Body.Close() }()

	raw, readErr := io.ReadAll(io.LimitReader(httpResp.Body, maxRespBytes))
	if readErr != nil {
		if ctx.Err() != nil {
			return nil, false, fmt.Errorf("gemini: embed read body: %w", readErr)
		}
		return nil, true, fmt.Errorf("gemini: embed read body: %w", readErr)
	}

	switch {
	case httpResp.StatusCode == http.StatusTooManyRequests || httpResp.StatusCode >= 500:
		return nil, true, fmt.Errorf("gemini: embed status %d: %s", httpResp.StatusCode, snippet(raw))
	case httpResp.StatusCode == http.StatusNotFound:
		// Тухайн model энэ түлхүүр/API хувилбарт байхгүй — дараагийн нэрийг
		// оролдоно (дахин оролдоод нэмэргүй).
		return nil, false, fmt.Errorf("%w: %s: %s", errEmbedModelUnavailable, model, snippet(raw))
	case httpResp.StatusCode >= 300:
		return nil, false, fmt.Errorf("gemini: embed status %d: %s", httpResp.StatusCode, snippet(raw))
	}

	var out batchEmbedResponse
	if jErr := json.Unmarshal(raw, &out); jErr != nil {
		return nil, false, fmt.Errorf("gemini: decode embed response: %w", jErr)
	}
	if len(out.Embeddings) != len(texts) {
		return nil, false, fmt.Errorf("%w: got %d vectors for %d texts", ErrEmbedShape, len(out.Embeddings), len(texts))
	}

	vectors = make([][]float32, 0, len(out.Embeddings))
	for i, e := range out.Embeddings {
		if len(e.Values) != EmbedDim {
			return nil, false, fmt.Errorf("%w: vector %d has %d dims, want %d", ErrEmbedShape, i, len(e.Values), EmbedDim)
		}
		vectors = append(vectors, e.Values)
	}
	return vectors, false, nil
}

// VectorLiteral нь векторыг pgvector-ийн текст хэлбэрт ("[0.1,0.2,…]")
// хөрвүүлнэ — pgx нь extension-ий төрлийг мэдэхгүй тул query-д ингэж
// дамжуулаад ::vector-оор cast хийнэ.
func VectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 12)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		// %g нь шаардлагагүй тэгүүдийг хасаж литералыг богиносгоно.
		fmt.Fprintf(&b, "%g", f)
	}
	b.WriteByte(']')
	return b.String()
}
