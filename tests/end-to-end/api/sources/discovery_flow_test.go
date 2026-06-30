package sources_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

// TestE2E_SourceDiscovery_DryRun walks the real flow: login → create a source whose RSS feed is
// mocked → call the discovery dry-run with a date override → see the new items returned, with
// nothing persisted.
func TestE2E_SourceDiscovery_DryRun(t *testing.T) {
	requireNotProduction(t)

	const feedURL = "https://e2e-disc.com/rss.xml"
	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		feedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	})

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-disc-1", Email: "disc@example.com", Name: "Disc User",
		},
	}
	app, queries := setupE2EApp(t, oauth, mockClient)
	token := loginViaCallback(t, app, queries)

	// Create the source through the API.
	createBody := `{"url":"https://e2e-disc.com","url_rss":"` + feedURL + `"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)

	// Dry-run discovery with an old lower bound → Fresh + Undated qualify.
	discReq, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id+"/discovery?last_article_discovery_at=2025-01-08T00:00:00Z", nil)
	discReq.Header.Set("Authorization", "Bearer "+token)
	discResp, err := app.Test(discReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, discResp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(discResp, &result))
	assert.Len(t, result["articles"].([]any), 2)

	// Nothing was persisted by the dry-run.
	system, err := queries.GetSystem(t.Context())
	require.NoError(t, err)
	assert.False(t, system.LastArticleDiscoveryAt.Valid)
}
