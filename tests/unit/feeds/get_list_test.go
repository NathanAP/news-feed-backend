package feeds_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func TestGetFeed_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/01900000-0000-7000-8000-000000000030", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Metallica Feed", result["name"])
}

func TestGetFeed_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _, _ string) (db.Feed, error) {
			return db.Feed{}, controllers.ErrFeedNotFound
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/other-user-feed", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetFeed_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/some-id", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListFeeds_Success(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, userID string) ([]db.Feed, error) {
			return []db.Feed{fixtures.NewTestFeed(userID), fixtures.NewTestFeedAlt(userID)}, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result, 2)
}

func TestListFeeds_Empty(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ string) ([]db.Feed, error) {
			return []db.Feed{}, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Empty(t, result)
}

func TestListFeeds_FilterByName(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, userID string) ([]db.Feed, error) {
			return []db.Feed{fixtures.NewTestFeed(userID), fixtures.NewTestFeedAlt(userID)}, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds?name=anime", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	require.Len(t, result, 1)
	assert.Equal(t, "Anime Feed", result[0]["name"])
}

func TestListFeeds_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
