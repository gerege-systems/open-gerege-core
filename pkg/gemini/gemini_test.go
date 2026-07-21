// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noSleep нь тестэд backoff хүлээлтийг алгасуулна.
func noSleep(_ context.Context, _ time.Duration) error { return nil }

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "test-key", "test-model")
	c.sleep = noSleep
	return c, srv
}

func TestGenerateContent(t *testing.T) {
	textBody := `{"candidates":[{"content":{"role":"model","parts":[{"text":"Сайн байна уу"}]},"finishReason":"STOP"}]}`
	fnBody := `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_server_time","args":{"tz":"UB"}}}]}}]}`

	tests := []struct {
		name        string
		handler     func(calls *int32) http.HandlerFunc
		wantErr     bool
		wantText    string
		wantCalls   int32
		wantFnCalls int
	}{
		{
			name: "success returns text",
			handler: func(calls *int32) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					atomic.AddInt32(calls, 1)
					assert.Equal(t, "test-key", r.Header.Get("x-goog-api-key"))
					assert.Contains(t, r.URL.Path, "/models/test-model:generateContent")
					_, _ = w.Write([]byte(textBody))
				}
			},
			wantText:  "Сайн байна уу",
			wantCalls: 1,
		},
		{
			name: "retries on 500 then succeeds",
			handler: func(calls *int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					if atomic.AddInt32(calls, 1) < 3 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_, _ = w.Write([]byte(textBody))
				}
			},
			wantText:  "Сайн байна уу",
			wantCalls: 3,
		},
		{
			name: "retries on 429 then succeeds",
			handler: func(calls *int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					if atomic.AddInt32(calls, 1) == 1 {
						w.WriteHeader(http.StatusTooManyRequests)
						return
					}
					_, _ = w.Write([]byte(textBody))
				}
			},
			wantText:  "Сайн байна уу",
			wantCalls: 2,
		},
		{
			name: "gives up after max attempts",
			handler: func(calls *int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(calls, 1)
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			},
			wantErr:   true,
			wantCalls: 3,
		},
		{
			name: "does not retry on 400",
			handler: func(calls *int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(calls, 1)
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
				}
			},
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name: "parses function calls",
			handler: func(calls *int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(calls, 1)
					_, _ = w.Write([]byte(fnBody))
				}
			},
			wantCalls:   1,
			wantFnCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int32
			client, _ := newTestClient(t, tt.handler(&calls))

			resp, err := client.GenerateContent(context.Background(), Request{
				Contents: []Content{{Role: "user", Parts: []Part{{Text: "hi"}}}},
			})

			assert.Equal(t, tt.wantCalls, atomic.LoadInt32(&calls))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantText != "" {
				assert.Equal(t, tt.wantText, resp.Text())
			}
			if tt.wantFnCalls > 0 {
				fnCalls := resp.FunctionCalls()
				require.Len(t, fnCalls, tt.wantFnCalls)
				assert.Equal(t, "get_server_time", fnCalls[0].Name)
				assert.Equal(t, "UB", fnCalls[0].Args["tz"])
			}
		})
	}
}

func TestGenerateContentNotConfigured(t *testing.T) {
	c := NewClient("", "", "")
	_, err := c.GenerateContent(context.Background(), Request{})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestResponseHelpersEmpty(t *testing.T) {
	var r Response
	assert.Empty(t, r.Text())
	assert.Nil(t, r.FunctionCalls())
	assert.Equal(t, "model", r.ModelContent().Role)
}
