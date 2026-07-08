package sources_test

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

func TestUpdateSource_Success(t *testing.T) {
	requireNotProduction(t)

	source := fixtures.NewTestSource()
	ctrl := &mockSourceCtrl{
		updateFn: func(_ context.Context, _ db.Querier, id, name, url, urlRss string) (db.Source, error) {
			s := source
			s.Name = name
			s.Url = url
			s.UrlRss = urlRss
			return s, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	body := `{"name":"Updated News","url":"https://updated.com","url_rss":"https://updated.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/"+source.ID, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Updated News", result["name"])
	assert.Equal(t, "https://updated.com", result["url"])
	assert.Equal(t, "https://updated.com/rss.xml", result["url_rss"])
}

func TestUpdateSource_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		updateFn: func(_ context.Context, _ db.Querier, _, _, _, _ string) (db.Source, error) {
			return db.Source{}, controllers.ErrSourceNotFound
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	body := `{"name":"Updated News","url":"https://updated.com","url_rss":"https://updated.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/non-existent", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateSource_Conflict(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		updateFn: func(_ context.Context, _ db.Querier, _, _, _, _ string) (db.Source, error) {
			return db.Source{}, controllers.ErrSourceAlreadyExists
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	body := `{"name":"Updated News","url":"https://duplicate.com","url_rss":"https://duplicate.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/some-id", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestUpdateSource_MissingName(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/some-id", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateSource_MissingURL(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Updated News","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/some-id", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateSource_InvalidURLFormat(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Updated News","url":"not-a-url","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/some-id", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateSource_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Updated News","url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/some-id", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
