// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// streamLineLimit — SSE-ийн нэг мөрийн дээд урт. Gemini-гийн chunk-ууд
// богино боловч inline media (audio) орж ирвэл том болно.
const streamLineLimit = 8 << 20

// StreamGenerateContent нь :streamGenerateContent?alt=sse-ийг дуудаж, ирсэн
// текстийн хэсгүүдийг onText руу шууд дамжуулна (хүлээхгүй). Эцэст нь бүх
// хэсгийг нэгтгэсэн Response буцаана — function calling-ийн давталт (usecase)
// үүнийг stream биш дуудалттай ижилхэн боловсруулна.
//
// Дахин оролдлого ХИЙХГҮЙ: эхний байт гарсны дараа дахин эхлүүлбэл хэрэглэгч
// хагас хариултыг хоёр удаа харна. Түр зуурын алдааг дуудагч (usecase)
// fallback мессежээр намжаана.
func (c *Client) StreamGenerateContent(ctx context.Context, req Request, onText func(string) error) (Response, error) {
	if c.apiKey == "" {
		return Response{}, ErrNotConfigured
	}

	buf, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("gemini: marshal stream request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", c.base, c.model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return Response{}, fmt.Errorf("gemini: build stream request: %w", err)
	}
	httpReq.Header.Set("x-goog-api-key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("%w: stream http: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return Response{}, fmt.Errorf("%w: stream status %d: %s", ErrUnavailable, resp.StatusCode, snippet(raw))
		}
		return Response{}, fmt.Errorf("gemini: stream status %d: %s", resp.StatusCode, snippet(raw))
	}

	return readSSE(resp.Body, onText)
}

// readSSE нь "data: {json}" мөрүүдийг уншиж, хэсгүүдийг нэг Response болгон
// хураана. Текстийн хэсэг ирэх бүрд onText дуудагдана.
func readSSE(body io.Reader, onText func(string) error) (Response, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), streamLineLimit)

	var parts []Part
	var finish string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var chunk Response
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			// Ганц эвдэрсэн chunk нь бүх урсгалыг унагах ёсгүй.
			continue
		}
		if len(chunk.Candidates) == 0 {
			continue
		}
		if chunk.Candidates[0].FinishReason != "" {
			finish = chunk.Candidates[0].FinishReason
		}
		for _, p := range chunk.Candidates[0].Content.Parts {
			parts = append(parts, p)
			if p.Text == "" || onText == nil {
				continue
			}
			if err := onText(p.Text); err != nil {
				return Response{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("%w: stream read: %v", ErrUnavailable, err)
	}

	return Response{Candidates: []Candidate{{
		Content:      Content{Role: "model", Parts: parts},
		FinishReason: finish,
	}}}, nil
}
