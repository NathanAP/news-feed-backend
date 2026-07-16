package sources_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

const discoveryFeedURL = "https://disc.com/rss.xml"

func seedDiscoverySource(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, token string) string {
	t.Helper()
	body := `{"name":"Disc News","url":"https://disc.com","url_rss":"` + discoveryFeedURL + `"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	require.NoError(t, readJSON(resp, &created))
	return created["id"].(string)
}

func TestIntegration_SourceDiscovery_DryRunReturnsItemsWithoutWriting(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		discoveryFeedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	})
	app, queries := setupIntegrationApp(t, mockClient)
	_, token := seedUser(t, queries)
	id := seedDiscoverySource(t, app, token)

	req, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id+"/article-discovery?last_article_discovery_at=2025-01-08T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result["articles"].([]any), 2) // Fresh + Undated; Old is filtered out

	// Dry-run must not touch the database: watermark stays unset and no article is created.
	system, err := queries.GetSystem(t.Context())
	require.NoError(t, err)
	assert.False(t, system.LastArticleDiscoveryAt.Valid, "watermark must remain unset after a dry-run")

	articles, err := queries.ListArticles(t.Context())
	require.NoError(t, err)
	assert.Empty(t, articles, "dry-run must not persist any article")
}

// TestIntegration_SourceDiscovery_DeduplicatesExisting proves the dry-run mirrors the CRON's
// url_original deduplication: an article already in the DB is excluded from the discovered set.
func TestIntegration_SourceDiscovery_DeduplicatesExisting(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		discoveryFeedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	})
	app, queries := setupIntegrationApp(t, mockClient)
	_, token := seedUser(t, queries)
	id := seedDiscoverySource(t, app, token)

	// Pre-create one of the feed's items (https://example.com/fresh) so it gets deduplicated.
	_, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID:          "01900000-0000-7000-8000-0000000000d1",
		Title:       "Fresh News",
		Content:     "# already here",
		UrlOriginal: "https://example.com/fresh",
		Keywords:    json.RawMessage(`["a","b","c","d","e"]`),
		SourceID:    id,
	})
	require.NoError(t, err)

	// No date override: all 3 feed items are candidates; the pre-existing one is dropped → 2 left.
	req, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id+"/article-discovery", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	articles := result["articles"].([]any)
	require.Len(t, articles, 2)
	for _, a := range articles {
		assert.NotEqual(t, "https://example.com/fresh", a.(map[string]any)["url_original"])
	}
}

func TestIntegration_SourceDiscovery_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	req, _ := http.NewRequest(http.MethodGet, "/v1/sources/non-existent/article-discovery", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
