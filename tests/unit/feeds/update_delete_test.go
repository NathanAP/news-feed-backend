package feeds_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func TestUpdateFeed_Success(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		updateFn: func(_ context.Context, _ db.Querier, _, userID, name string, _ []string) (db.Feed, error) {
			f := fixtures.NewTestFeed(userID)
			f.Name = name
			return f, nil
		},
	}
	app := buildApp(ctrl)
	body := `{"name":"Renamed Feed","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/feeds/01900000-0000-7000-8000-000000000030", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Renamed Feed", result["name"])
}

func TestUpdateFeed_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		updateFn: func(_ context.Context, _ db.Querier, _, _, _ string, _ []string) (db.Feed, error) {
			return db.Feed{}, controllers.ErrFeedNotFound
		},
	}
	app := buildApp(ctrl)
	body := `{"name":"X","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/feeds/other-user-feed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateFeed_TooFewKeywords(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"X","keywords":["a","b"]}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/feeds/some-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateFeed_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"X","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/feeds/some-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestDeleteFeed_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/01900000-0000-7000-8000-000000000030", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestDeleteFeed_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _, _ string) (db.Feed, error) {
			return db.Feed{}, controllers.ErrFeedNotFound
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/other-user-feed", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteFeed_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/some-id", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
