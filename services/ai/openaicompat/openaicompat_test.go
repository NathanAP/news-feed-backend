package openaicompat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/ai/openaicompat"
)

// chatServer returns a fake OpenAI-compatible endpoint that replies with the given assistant
// content (or a status code when status != 200).
func chatServer(t *testing.T, content string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":` + jsonString(content) + `}}]}`))
	}))
}

// jsonString marshals a string to a JSON string literal (with quotes/escaping).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestKeywords_StripsThinkingAndParses(t *testing.T) {
	srv := chatServer(t, "<think>let me pick terms</think>\n[\"alpha\",\"beta\",\"gamma\",\"delta\",\"epsilon\"]", http.StatusOK)
	defer srv.Close()

	client := openaicompat.NewClient(srv.URL, "qwen3:4b")
	kw, err := client.Keywords(context.Background(), "Title", "content")
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma", "delta", "epsilon"}, kw)
}

func TestTreat_StripsThinking(t *testing.T) {
	srv := chatServer(t, "<think>reasoning here</think>\n# Treated body", http.StatusOK)
	defer srv.Close()

	client := openaicompat.NewClient(srv.URL, "qwen3:4b")
	treated, err := client.Treat(context.Background(), "Title", "content")
	require.NoError(t, err)
	assert.Equal(t, "# Treated body", treated)
}

func TestKeywords_ServerError(t *testing.T) {
	srv := chatServer(t, "", http.StatusInternalServerError)
	defer srv.Close()

	client := openaicompat.NewClient(srv.URL, "qwen3:4b")
	_, err := client.Keywords(context.Background(), "Title", "content")
	assert.Error(t, err)
}
