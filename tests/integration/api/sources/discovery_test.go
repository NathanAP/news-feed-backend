package sources_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

const discoveryFeedURL = "https://disc.com/rss.xml"

func seedDiscoverySource(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, token string) string {
	t.Helper()
	body := `{"url":"https://disc.com","url_rss":"` + discoveryFeedURL + `"}`
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

	req, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id+"/discovery?last_article_discovery_at=2025-01-08T00:00:00Z", nil)
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

func TestIntegration_SourceDiscovery_WithoutOverrideUsesWatermark(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		discoveryFeedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	})
	app, queries := setupIntegrationApp(t, mockClient)
	_, token := seedUser(t, queries)
	id := seedDiscoverySource(t, app, token)

	// No override: watermark is NULL → treated as "now" → only the undated item qualifies.
	req, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id+"/discovery", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	articles := result["articles"].([]any)
	require.Len(t, articles, 1)
	assert.Equal(t, "Undated News", articles[0].(map[string]any)["title"])
}

func TestIntegration_SourceDiscovery_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	req, _ := http.NewRequest(http.MethodGet, "/v1/sources/non-existent/discovery", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
