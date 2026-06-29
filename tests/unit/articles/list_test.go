package articles_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func listArticles(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, query string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/v1/articles"+query, nil)
	require.NoError(t, err)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestListArticles_Success(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Article, error) {
			return []db.Article{fixtures.NewTestArticle(), fixtures.NewTestArticleAlt()}, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result, 2)
}

func TestListArticles_Empty(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Article, error) {
			return []db.Article{}, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Empty(t, result)
}

func TestListArticles_FilterByURL(t *testing.T) {
	requireNotProduction(t)

	a1 := fixtures.NewTestArticle()    // url: https://example.com/news/test-article
	a2 := fixtures.NewTestArticleAlt() // url: https://other-site.com/news/another
	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Article, error) {
			return []db.Article{a1, a2}, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "?url=other-site", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	require.Len(t, result, 1)
	assert.Equal(t, a2.UrlOriginal, result[0]["url_original"])
}

func TestListArticles_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier) ([]db.Article, error) {
			return nil, assert.AnError
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "", true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestListArticles_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := listArticles(t, app, "", false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
