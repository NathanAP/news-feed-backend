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
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func putArticle(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, id, body string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, "/v1/articles/"+id, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestUpdateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	article := fixtures.NewTestArticle()
	ctrl := &mockArticleCtrl{
		updateFn: func(_ context.Context, _ db.Querier, id, title, content, urlOriginal string, keywords []string) (db.Article, error) {
			a := article
			a.Title = title
			return a, nil
		},
	}
	app := buildApp(ctrl)

	body := `{"title":"Updated","content":"C","url_original":"https://e.com/u","keywords":["a","b","c","d","e"]}`
	resp := putArticle(t, app, article.ID, body, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Updated", result["title"])
}

func TestUpdateArticle_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		updateFn: func(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleNotFound
		},
	}
	app := buildApp(ctrl)

	body := `{"title":"T","content":"C","url_original":"https://e.com/u","keywords":["a","b","c","d","e"]}`
	resp := putArticle(t, app, "missing", body, true)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateArticle_Conflict(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		updateFn: func(_ context.Context, _ db.Querier, _, _, _, _ string, _ []string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleAlreadyExists
		},
	}
	app := buildApp(ctrl)

	body := `{"title":"T","content":"C","url_original":"https://e.com/dup","keywords":["a","b","c","d","e"]}`
	resp := putArticle(t, app, "some-id", body, true)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestUpdateArticle_TooFewKeywords(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/u","keywords":["a","b"]}`
	resp := putArticle(t, app, "some-id", body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateArticle_InvalidURL(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"bad","keywords":["a","b","c","d","e"]}`
	resp := putArticle(t, app, "some-id", body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	body := `{"title":"T","content":"C","url_original":"https://e.com/u","keywords":["a","b","c","d","e"]}`
	resp := putArticle(t, app, "some-id", body, false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
