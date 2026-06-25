package sources_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

func TestRSSDiscovery_FindsFeedViaHTMLLink(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://example.com": {
			StatusCode: http.StatusOK,
			Body:       external.SampleHTMLWithRSSLink,
		},
		"https://example.com/rss.xml": {
			StatusCode: http.StatusOK,
			Body:       external.SampleRSSFeed,
		},
	})
	app := buildApp(&mockSourceCtrl{}, mockClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://example.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds, ok := result["feeds"].([]any)
	require.True(t, ok)
	assert.Len(t, feeds, 1)
	assert.Equal(t, "https://example.com/rss.xml", feeds[0])
}

func TestRSSDiscovery_FindsFeedViaCommonPath(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://example.com": {
			StatusCode: http.StatusOK,
			Body:       external.SampleHTMLNoFeed,
		},
		"https://example.com/rss": {
			StatusCode: http.StatusOK,
			Body:       external.SampleRSSFeed,
		},
	})
	app := buildApp(&mockSourceCtrl{}, mockClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://example.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds, ok := result["feeds"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, feeds)
}

func TestRSSDiscovery_NoFeedsFound(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://example.com": {
			StatusCode: http.StatusOK,
			Body:       external.SampleHTMLNoFeed,
		},
	})
	app := buildApp(&mockSourceCtrl{}, mockClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://example.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds, ok := result["feeds"].([]any)
	require.True(t, ok)
	assert.Empty(t, feeds)
}

func TestRSSDiscovery_MissingURLParam(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRSSDiscovery_InvalidURL(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=not-a-url", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRSSDiscovery_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://example.com", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRSSDiscovery_HTTPErrorReturnsEmptyFeeds(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://unreachable.com": {Err: assert.AnError},
	})
	app := buildApp(&mockSourceCtrl{}, mockClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://unreachable.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds := result["feeds"].([]any)
	assert.Empty(t, feeds)
}
