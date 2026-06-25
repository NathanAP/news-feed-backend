package sources_test

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

func TestGetSource_Success(t *testing.T) {
	requireNotProduction(t)

	source := fixtures.NewTestSource()
	ctrl := &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, id string) (db.Source, error) {
			assert.Equal(t, source.ID, id)
			return source, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/"+source.ID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, source.ID, result["id"])
	assert.Equal(t, source.Url, result["url"])
	assert.Equal(t, source.UrlRss, result["url_rss"])
}

func TestGetSource_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Source, error) {
			return db.Source{}, controllers.ErrSourceNotFound
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/non-existent-id", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetSource_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Source, error) {
			return db.Source{}, assert.AnError
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/some-id", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestGetSource_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/sources/some-id", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
