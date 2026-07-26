// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/http/middlewares"
)

// deadlineOf нь handler-т ирсэн context-ийн үлдсэн хугацааг буцаана.
func deadlineOf(t *testing.T, path string) time.Duration {
	t.Helper()
	var left time.Duration
	h := middlewares.TimeoutMiddleware(middlewares.DefaultRequestTimeout)(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			dl, ok := r.Context().Deadline()
			require.True(t, ok, "deadline тавигдсан байх ёстой")
			left = time.Until(dl)
		}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, http.NoBody))
	return left
}

// AI endpoint-ууд (Gemini дуудлага 10-20 секунд) ерөнхий 30с-д багтдаггүй тул
// өөрийн уртавтар deadline-тай.
func TestTimeoutMiddleware_AIPathsGetLongerDeadline(t *testing.T) {
	ai := deadlineOf(t, "/api/v1/ai/tts")
	other := deadlineOf(t, "/api/v1/users/me")

	assert.Greater(t, ai, middlewares.DefaultRequestTimeout,
		"AI зам ерөнхий хязгаараас урт байх ёстой")
	assert.LessOrEqual(t, ai, middlewares.AIRequestTimeout)
	assert.LessOrEqual(t, other, middlewares.DefaultRequestTimeout,
		"бусад зам ерөнхий хязгаартаа үлдэнэ")
}

// nginx-ийн proxy_read_timeout (60с)-аас доогуур байх ёстой — эс тэгвээс
// клиент цэвэр хариу биш, тасарсан холболт хүлээж авна.
func TestAIRequestTimeout_FitsUnderProxyTimeout(t *testing.T) {
	assert.Less(t, middlewares.AIRequestTimeout, 60*time.Second)
}
