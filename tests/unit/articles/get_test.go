package articles_test

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

func getArticle(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, id string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/v1/articles/"+id, nil)
	require.NoError(t, err)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestGetArticle_Success(t *testing.T) {
	requireNotProduction(t)

	article := fixtures.NewTestArticle()
	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, id string) (db.Article, error) {
			assert.Equal(t, article.ID, id)
			return article, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := getArticle(t, app, article.ID, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, article.ID, result["id"])
	assert.Equal(t, article.Title, result["title"])
	keywords := result["keywords"].([]any)
	assert.Len(t, keywords, 5)
}

func TestGetArticle_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleNotFound
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := getArticle(t, app, "missing-id", true)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetArticle_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			return db.Article{}, assert.AnError
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := getArticle(t, app, "some-id", true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestGetArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := getArticle(t, app, "some-id", false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
