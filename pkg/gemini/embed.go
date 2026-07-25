// Gerege Template Platform V3.0
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

// defaultEmbedModel нь embedding-ийн өгөгдмөл model.
const defaultEmbedModel = "text-embedding-004"

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
	Model    string       `json:"model"`
	Content  embedContent `json:"content"`
	TaskType string       `json:"taskType,omitempty"`
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
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff << (attempt - 1)
			if err := c.sleep(ctx, backoff); err != nil {
				return nil, fmt.Errorf("gemini: retry wait: %w", err)
			}
		}

		out, retryable, err := c.embedOnce(ctx, texts, taskType)
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

func (c *Client) embedOnce(ctx context.Context, texts []string, taskType string) (vectors [][]float32, retryable bool, err error) {
	model := c.embedModel
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
