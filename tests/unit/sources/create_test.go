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

func TestCreateSource_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Test Source","url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
	assert.Equal(t, "Test Source", result["name"])
	assert.Equal(t, "https://example.com", result["url"])
	assert.Equal(t, "https://example.com/rss.xml", result["url_rss"])
}

func TestCreateSource_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Example News","url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateSource_MissingName(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSource_NameTooLong(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"` + strings.Repeat("a", 121) + `","url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSource_MissingURL(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Example News","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSource_MissingURLRss(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Example News","url":"https://example.com"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSource_InvalidURL(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Example News","url":"not-a-url","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSource_InvalidURLRss(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"name":"Example News","url":"https://example.com","url_rss":"ftp://example.com/rss"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateSource_Conflict(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _, _ string) (db.Source, error) {
			return db.Source{}, controllers.ErrSourceAlreadyExists
		},
	}
	app := buildApp(ctrl, http.DefaultClient)
	body := `{"name":"Example News","url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateSource_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _, _ string) (db.Source, error) {
			return db.Source{}, assert.AnError
		},
	}
	app := buildApp(ctrl, http.DefaultClient)
	body := `{"name":"Example News","url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCreateSource_InvalidBody(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader("not json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	// fiber parses gracefully, missing fields trigger field-level validation
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	_ = fixtures.NewTestSource()
}
