package sources_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

const discoveryFeedURL = "https://feed.example.com/rss.xml"

// sourceWithFeed returns a mock source controller whose source points at discoveryFeedURL.
func sourceWithFeed() *mockSourceCtrl {
	return &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, id string) (db.Source, error) {
			return db.Source{ID: id, Status: 1, Url: "https://example.com", UrlRss: discoveryFeedURL}, nil
		},
	}
}

func discoveryGet(t *testing.T, ctrl controllers.SourceControllerInterface, responses map[string]external.MockRSSResponse, query string) *http.Response {
	t.Helper()
	app := buildApp(ctrl, external.NewMockRSSClient(responses))
	target := "/v1/sources/src-1/discovery"
	if query != "" {
		target += "?" + query
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))
	// Generous timeout: the feed-error path retries with exponential backoff (~1.4s).
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	return resp
}

func TestSourceDiscovery_ReturnsNewItems(t *testing.T) {
	requireNotProduction(t)

	responses := map[string]external.MockRSSResponse{
		discoveryFeedURL: {StatusCode: http.StatusOK, Body: external.SampleRSSFeedDated},
	}
	// since 2025-01-08 → "Old News" (06 Jan) excluded; "Fresh News" (10 Jan) and "Undated News" in.
	resp := discoveryGet(t, sourceWithFeed(), responses, "last_article_discovery_at=2025-01-08T00:00:00Z")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	articles := result["articles"].([]any)
	require.Len(t, articles, 2)

	titles := map[string]bool{}
	for _, a := range articles {
		titles[a.(map[string]any)["title"].(string)] = true
	}
	assert.True(t, titles["Fresh News"])
	assert.True(t, titles["Undated News"])
	assert.False(t, titles["Old News"])
}

func TestSourceDiscovery_EmptyFeed(t *testing.T) {
	requireNotProduction(t)

	emptyFeed := `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Empty</title><link>https://example.com</link><description>none</description></channel></rss>`
	responses := map[string]external.MockRSSResponse{
		discoveryFeedURL: {StatusCode: http.StatusOK, Body: emptyFeed},
	}
	resp := discoveryGet(t, sourceWithFeed(), responses, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Empty(t, result["articles"].([]any))
}

func TestSourceDiscovery_SourceNotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Source, error) {
			return db.Source{}, controllers.ErrSourceNotFound
		},
	}
	resp := discoveryGet(t, ctrl, nil, "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSourceDiscovery_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := buildApp(sourceWithFeed(), http.DefaultClient)
	req, err := http.NewRequest(http.MethodGet, "/v1/sources/src-1/discovery", nil)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSourceDiscovery_InvalidDateQuery(t *testing.T) {
	requireNotProduction(t)

	resp := discoveryGet(t, sourceWithFeed(), nil, "last_article_discovery_at=not-a-date")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSourceDiscovery_FeedError(t *testing.T) {
	requireNotProduction(t)

	responses := map[string]external.MockRSSResponse{
		discoveryFeedURL: {Err: assert.AnError},
	}
	resp := discoveryGet(t, sourceWithFeed(), responses, "last_article_discovery_at=2025-01-08T00:00:00Z")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
