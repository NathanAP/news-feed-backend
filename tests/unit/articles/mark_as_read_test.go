package articles_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

func markAsRead(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, id string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, "/v1/articles/"+id+"/read", nil)
	require.NoError(t, err)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestMarkAsRead_HasFeed_Returns200(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockArticleFeedCtrl{
		markAsReadFn: func(_ context.Context, _ db.Querier, _, _ string) (bool, error) {
			return true, nil
		},
	}
	app := buildApp(&mockArticleCtrl{}, afCtrl)
	resp := markAsRead(t, app, fixtures.NewTestArticle().ID, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMarkAsRead_NoFeed_Returns204(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockArticleFeedCtrl{
		markAsReadFn: func(_ context.Context, _ db.Querier, _, _ string) (bool, error) {
			return false, nil
		},
	}
	app := buildApp(&mockArticleCtrl{}, afCtrl)
	resp := markAsRead(t, app, fixtures.NewTestArticle().ID, true)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestMarkAsRead_ArticleNotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _ string) (db.Article, error) {
			return db.Article{}, controllers.ErrArticleNotFound
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})
	resp := markAsRead(t, app, "missing-id", true)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestMarkAsRead_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := markAsRead(t, app, "some-id", false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── GET enrichment tests ──────────────────────────────────────────────────────

func TestGetArticle_IsReadNilWhenNoFeed(t *testing.T) {
	requireNotProduction(t)

	// default afCtrl returns empty records → is_read must be null
	app := defaultApp()
	resp := getArticle(t, app, fixtures.NewTestArticle().ID, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Nil(t, result["is_read"])
}

func TestGetArticle_IsReadFalseWhenUnread(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockArticleFeedCtrl{
		findByArticleAndUserFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ArticlesFeed, error) {
			return []db.ArticlesFeed{
				{ID: "af-1", ArticleID: "a1", FeedID: "f1", IsRead: false, CreatedAt: time.Now()},
			}, nil
		},
	}
	app := buildApp(&mockArticleCtrl{}, afCtrl)
	resp := getArticle(t, app, fixtures.NewTestArticle().ID, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, false, result["is_read"])
}

func TestGetArticle_IsReadTrueWhenAllRead(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockArticleFeedCtrl{
		findByArticleAndUserFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ArticlesFeed, error) {
			return []db.ArticlesFeed{
				{ID: "af-1", IsRead: true, CreatedAt: time.Now()},
				{ID: "af-2", IsRead: true, CreatedAt: time.Now(), ModifiedAt: sql.NullTime{Valid: true, Time: time.Now()}},
			}, nil
		},
	}
	app := buildApp(&mockArticleCtrl{}, afCtrl)
	resp := getArticle(t, app, fixtures.NewTestArticle().ID, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, true, result["is_read"])
}

func TestGetArticle_IsReadFalseWhenMixedReadState(t *testing.T) {
	requireNotProduction(t)

	// One feed read, another unread → result should be false (not fully read)
	afCtrl := &mockArticleFeedCtrl{
		findByArticleAndUserFn: func(_ context.Context, _ db.Querier, _, _ string) ([]db.ArticlesFeed, error) {
			return []db.ArticlesFeed{
				{ID: "af-1", IsRead: true, CreatedAt: time.Now()},
				{ID: "af-2", IsRead: false, CreatedAt: time.Now()},
			}, nil
		},
	}
	app := buildApp(&mockArticleCtrl{}, afCtrl)
	resp := getArticle(t, app, fixtures.NewTestArticle().ID, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, false, result["is_read"])
}
