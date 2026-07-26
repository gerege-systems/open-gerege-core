// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vectorsOf нь тестийн хариунд EmbedDim урттай вектор бэлдэнэ.
func vectorsOf(n int, fill float32) batchEmbedResponse {
	out := batchEmbedResponse{Embeddings: make([]embedValues, 0, n)}
	for i := 0; i < n; i++ {
		vals := make([]float32, EmbedDim)
		for j := range vals {
			vals[j] = fill
		}
		out.Embeddings = append(out.Embeddings, embedValues{Values: vals})
	}
	return out
}

func TestEmbed_Success(t *testing.T) {
	var gotPath string
	var gotBody batchEmbedRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(vectorsOf(2, 0.25)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	got, err := c.Embed(context.Background(), []string{"нэг", "хоёр"}, TaskDocument)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Len(t, got[0], EmbedDim)
	assert.Equal(t, float32(0.25), got[0][0])
	// Өгөгдмөл embedding model + batch endpoint рүү явсан байх ёстой.
	assert.Equal(t, "/models/"+defaultEmbedModel+":batchEmbedContents", gotPath)
	require.Len(t, gotBody.Requests, 2)
	assert.Equal(t, TaskDocument, gotBody.Requests[0].TaskType)
	assert.Equal(t, "нэг", gotBody.Requests[0].Content.Parts[0].Text)
}

func TestEmbed_UsesConfiguredModel(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, json.NewEncoder(w).Encode(vectorsOf(1, 1)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "").WithEmbedModel("gemini-embedding-001")
	_, err := c.Embed(context.Background(), []string{"x"}, TaskQuery)

	require.NoError(t, err)
	assert.Equal(t, "/models/gemini-embedding-001:batchEmbedContents", gotPath)
}

func TestEmbed_NotConfigured(t *testing.T) {
	c := NewClient("http://unused", "", "")
	_, err := c.Embed(context.Background(), []string{"x"}, TaskQuery)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestEmbed_EmptyInputSkipsCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("хоосон оролтод HTTP дуудалт хийх ёсгүй")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	got, err := c.Embed(context.Background(), nil, TaskDocument)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEmbed_RetriesOn5xxThenSucceeds(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(vectorsOf(1, 0.5)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	c.sleep = func(context.Context, time.Duration) error { return nil } // тестэд хүлээхгүй

	got, err := c.Embed(context.Background(), []string{"x"}, TaskQuery)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, calls)
}

func TestEmbed_NoRetryOn4xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	c.sleep = func(context.Context, time.Duration) error { return nil }

	_, err := c.Embed(context.Background(), []string{"x"}, TaskQuery)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "4xx дээр дахин оролдох ёсгүй")
}

func TestEmbed_RejectsWrongShape(t *testing.T) {
	tests := []struct {
		name string
		resp batchEmbedResponse
	}{
		{name: "вектор дутуу", resp: vectorsOf(1, 1)},
		{name: "хэмжээ буруу", resp: batchEmbedResponse{Embeddings: []embedValues{
			{Values: []float32{1, 2, 3}}, {Values: []float32{1, 2, 3}},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(tt.resp))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "key", "")
			_, err := c.Embed(context.Background(), []string{"a", "b"}, TaskDocument)
			assert.True(t, errors.Is(err, ErrEmbedShape), "ErrEmbedShape хүлээж байна, гарсан: %v", err)
		})
	}
}

func TestVectorLiteral(t *testing.T) {
	assert.Equal(t, "[0.5,-1,0]", VectorLiteral([]float32{0.5, -1, 0}))
	assert.Equal(t, "[]", VectorLiteral(nil))
	// pgvector-ийн литерал хэлбэрт хаалт болон таслал л байна (зай биш).
	assert.False(t, strings.Contains(VectorLiteral([]float32{1, 2}), " "))
}

// Тохируулсан / өгөгдмөл model нь тухайн түлхүүрт байхгүй (404) бол дараагийн
// нэр рүү шилжинэ; ажилласан нэрийг тогтоож аваад дараагийн дуудалтад шууд
// хэрэглэнэ (дахин 404 идэхгүй).
func TestEmbed_FallsBackWhenModelMissing(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, embedModelFallbacks[0]) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"not found for API version v1beta"}}`))
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(vectorsOf(1, 0.3)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	c.sleep = func(context.Context, time.Duration) error { return nil }

	got, err := c.Embed(context.Background(), []string{"x"}, TaskQuery)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, paths, 2, "эхнийх 404 → хоёр дахийг оролдоно")
	assert.Contains(t, paths[0], embedModelFallbacks[0])
	assert.Contains(t, paths[1], embedModelFallbacks[1])

	// Хоёр дахь дуудалт нь ажилласан model руу шууд очно.
	_, err = c.Embed(context.Background(), []string{"y"}, TaskQuery)
	require.NoError(t, err)
	require.Len(t, paths, 3)
	assert.Contains(t, paths[2], embedModelFallbacks[1])
}

// Бүх нэр 404 бол сүүлчийн алдааг буцаана (чимээгүй амжилт биш).
func TestEmbed_AllModelsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	c.sleep = func(context.Context, time.Duration) error { return nil }

	_, err := c.Embed(context.Background(), []string{"x"}, TaskQuery)
	require.Error(t, err)
	assert.ErrorIs(t, err, errEmbedModelUnavailable)
}

// Хүсэлт бүрд DB-ийн багана (vector(768))-тай таарах хэмжээг шууд заана —
// gemini-embedding-001 нь өгөгдмөлөөр 3072 хэмжээст гардаг.
func TestEmbed_RequestsFixedDimension(t *testing.T) {
	var body batchEmbedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NoError(t, json.NewEncoder(w).Encode(vectorsOf(1, 0.1)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "")
	_, err := c.Embed(context.Background(), []string{"x"}, TaskDocument)

	require.NoError(t, err)
	require.Len(t, body.Requests, 1)
	assert.Equal(t, EmbedDim, body.Requests[0].OutputDimensionality)
}
