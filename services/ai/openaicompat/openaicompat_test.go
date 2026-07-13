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

	client := openaicompat.NewClient(srv.URL, "qwen3:4b", "")
	kw, err := client.Keywords(context.Background(), "Title", "content")
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma", "delta", "epsilon"}, kw)
}

func TestKeywords_SendsAuthHeaderWhenKeySet(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"[\"alpha\",\"beta\",\"gamma\",\"delta\",\"epsilon\"]"}}]}`))
	}))
	defer srv.Close()

	client := openaicompat.NewClient(srv.URL, "some-model", "secret-key")
	_, err := client.Keywords(context.Background(), "Title", "content")
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret-key", gotAuth)
}

func TestKeywords_BaseURLWithV1DoesNotDouble(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"[\"alpha\",\"beta\",\"gamma\",\"delta\",\"epsilon\"]"}}]}`))
	}))
	defer srv.Close()

	// Base URL already ends with /v1 (like Groq): must not produce /v1/v1/chat/completions.
	client := openaicompat.NewClient(srv.URL+"/v1", "some-model", "")
	_, err := client.Keywords(context.Background(), "Title", "content")
	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", gotPath)
}

func TestKeywords_ServerError(t *testing.T) {
	srv := chatServer(t, "", http.StatusInternalServerError)
	defer srv.Close()

	client := openaicompat.NewClient(srv.URL, "qwen3:4b", "")
	_, err := client.Keywords(context.Background(), "Title", "content")
	assert.Error(t, err)
}
