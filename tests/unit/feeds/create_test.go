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
)

func postCreate(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, body string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestCreateFeed_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
	assert.Equal(t, "Metallica Feed", result["name"])
	assert.NotEmpty(t, result["user_id"])
	keywords := result["keywords"].([]any)
	assert.Len(t, keywords, 5)
}

func TestCreateFeed_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, validCreateBody(), false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateFeed_MissingName(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, `{"keywords":["a","b","c","d","e"]}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateFeed_NameTooLong(t *testing.T) {
	requireNotProduction(t)

	longName := strings.Repeat("a", 121)
	app := defaultApp()
	resp := postCreate(t, app, `{"name":"`+longName+`","keywords":["a","b","c","d","e"]}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateFeed_TooFewKeywords(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, `{"name":"F","keywords":["a","b","c"]}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateFeed_TooManyKeywords(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, `{"name":"F","keywords":["k1","k2","k3","k4","k5","k6","k7","k8","k9","k10","k11","k12","k13","k14","k15","k16","k17","k18","k19","k20","k21"]}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateFeed_EmptyKeyword(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, `{"name":"F","keywords":["a","","c","d","e"]}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateFeed_LimitReached(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _ string, _ []string) (db.Feed, error) {
			return db.Feed{}, controllers.ErrFeedLimitReached
		},
	}
	app := buildApp(ctrl)
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateFeed_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _ string, _ []string) (db.Feed, error) {
			return db.Feed{}, assert.AnError
		},
	}
	app := buildApp(ctrl)
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCreateFeed_InvalidBody(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, "not json", true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
