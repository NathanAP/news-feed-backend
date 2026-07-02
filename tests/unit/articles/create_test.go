package articles_test

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
	req, err := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestCreateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
	assert.Equal(t, "Test Article", result["title"])
	keywords, ok := result["keywords"].([]any)
	require.True(t, ok)
	assert.Len(t, keywords, 5)
}

func TestCreateArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, validCreateBody(), false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateArticle_MissingTitle(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"content":"x","url_original":"https://e.com/a","keywords":["a","b","c","d","e"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_MissingContent(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","url_original":"https://e.com/a","keywords":["a","b","c","d","e"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_InvalidURL(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"not-a-url","keywords":["a","b","c","d","e"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_TooFewKeywords(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/a","keywords":["a","b","c"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_TooManyKeywords(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/a","keywords":["k1","k2","k3","k4","k5","k6","k7","k8","k9","k10","k11","k12","k13","k14","k15","k16","k17","k18","k19","k20","k21"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_EmptyKeyword(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/a","keywords":["a","","c","d","e"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_Conflict(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string, _ *string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleAlreadyExists
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateArticle_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string, _ *string) (db.Article, error) {
			return db.Article{}, assert.AnError
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCreateArticle_MissingSourceID(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/a","keywords":["a","b","c","d","e"]}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_SourceInvalid(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		createFn: func(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string, _ *string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleSourceInvalid
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})
	resp := postCreate(t, app, validCreateBody(), true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_ResponseIncludesSourceID(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, validCreateBody(), true)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["source_id"])
}

func TestCreateArticle_MissingLanguageOriginal(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/a","keywords":["a","b","c","d","e"],"source_id":"01900000-0000-7000-8000-000000000010"}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_InvalidLanguageOriginal(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/a","keywords":["a","b","c","d","e"],"source_id":"01900000-0000-7000-8000-000000000010","language_original":"xx"}`
	resp := postCreate(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateArticle_InvalidBody(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := postCreate(t, app, "not json", true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
