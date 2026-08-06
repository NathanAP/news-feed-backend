package articles_test

import (
	"context"
	"github.com/gofiber/fiber/v3"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func deleteArticle(t *testing.T, app interface {
	Test(*http.Request, ...fiber.TestConfig) (*http.Response, error)
}, id string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, "/v1/articles/"+id, nil)
	require.NoError(t, err)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestDeleteArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := deleteArticle(t, app, fixtures.NewTestArticle().ID, true)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestDeleteArticle_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleNotFound
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := deleteArticle(t, app, "missing", true)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteArticle_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			return fixtures.NewTestArticle(), nil
		},
		softDeleteFn: func(_ context.Context, _ db.Querier, _ string) error {
			return assert.AnError
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := deleteArticle(t, app, "some-id", true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestDeleteArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := deleteArticle(t, app, "some-id", false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
