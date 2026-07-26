// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sseServer нь өгсөн мөрүүдийг SSE хэлбэрээр цацна.
func sseServer(t *testing.T, frames ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, f := range frames {
			_, _ = w.Write([]byte("data: " + f + "\n\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		}
	}))
}

func TestStreamGenerateContent_CollectsDeltas(t *testing.T) {
	srv := sseServer(t,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"Сайн "}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"байна уу"}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}]}`,
	)
	defer srv.Close()

	var deltas []string
	c := NewClient(srv.URL, "key", "")
	resp, err := c.StreamGenerateContent(context.Background(), Request{}, func(s string) error {
		deltas = append(deltas, s)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"Сайн ", "байна уу"}, deltas, "хэсэг бүр ирмэгц дамжина")
	assert.Equal(t, "Сайн байна уу", resp.Text())
	assert.Equal(t, "STOP", resp.Candidates[0].FinishReason)
}

func TestStreamGenerateContent_KeepsFunctionCalls(t *testing.T) {
	srv := sseServer(t,
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search_knowledge","args":{"query":"eID"}}}]}}]}`,
	)
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	resp, err := c.StreamGenerateContent(context.Background(), Request{}, nil)

	require.NoError(t, err)
	calls := resp.FunctionCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "search_knowledge", calls[0].Name)
}

// Эвдэрсэн ганц chunk нь бүх урсгалыг унагах ёсгүй.
func TestStreamGenerateContent_SkipsBadChunk(t *testing.T) {
	srv := sseServer(t, `{ энэ бол JSON биш }`, `{"candidates":[{"content":{"parts":[{"text":"за"}]}}]}`)
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	resp, err := c.StreamGenerateContent(context.Background(), Request{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "за", resp.Text())
}

func TestStreamGenerateContent_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"busy"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	_, err := c.StreamGenerateContent(context.Background(), Request{}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable, "5xx нь түр зуурын саатал")
}

func TestStreamGenerateContent_NotConfigured(t *testing.T) {
	c := NewClient("http://unused", "", "")
	_, err := c.StreamGenerateContent(context.Background(), Request{}, nil)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

// URL нь SSE горимыг заасан байх ёстой.
func TestStreamGenerateContent_UsesSSEEndpoint(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "text/event-stream")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "gemini-2.5-flash")
	_, err := c.StreamGenerateContent(context.Background(), Request{}, nil)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(path, ":streamGenerateContent"), "got %s", path)
	assert.Equal(t, "alt=sse", query)
}
