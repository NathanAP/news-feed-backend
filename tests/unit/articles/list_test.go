package articles_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

// Filtering and pagination run in SQL, so these unit tests assert what the handler is actually
// responsible for: turning the query string into the right filter, and shaping whatever the
// controller returns into the paginated envelope. That the filter really narrows rows is proven
// against a real Postgres in tests/integration/api/articles.

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
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListArticlesFilter) ([]db.Article, int64, error) {
			return []db.Article{fixtures.NewTestArticle(), fixtures.NewTestArticleAlt()}, 2, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Len(t, result, 2)
}

func TestListArticles_Empty(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListArticlesFilter) ([]db.Article, int64, error) {
			return []db.Article{}, 0, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Empty(t, result)
}

// The url query must reach the controller verbatim: it is the SQL layer that lowercases and matches,
// so the handler must not pre-process it (a handler that lowercased here would be harmless, but one
// that dropped or renamed it would silently return unfiltered data).
func TestListArticles_ForwardsURLFilter(t *testing.T) {
	requireNotProduction(t)

	var got controllers.ListArticlesFilter
	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier, filter controllers.ListArticlesFilter) ([]db.Article, int64, error) {
			got = filter
			return []db.Article{}, 0, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "?url=Other-Site", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Other-Site", got.URL)
}

// An absent url query must reach the controller as the empty string, which is what the controller
// maps to a NULL param (= no filter). Anything else would filter the list down to nothing.
func TestListArticles_NoFilterWhenURLAbsent(t *testing.T) {
	requireNotProduction(t)

	var got controllers.ListArticlesFilter
	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier, filter controllers.ListArticlesFilter) ([]db.Article, int64, error) {
			got = filter
			return []db.Article{}, 0, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	listArticles(t, app, "", true)
	assert.Empty(t, got.URL)
}

// The page/page_size query must arrive as the normalized (clamped) request the SQL LIMIT/OFFSET is
// built from — the handler is what converts a raw query into it.
func TestListArticles_ForwardsPaginationParams(t *testing.T) {
	requireNotProduction(t)

	tests := []struct {
		name     string
		query    string
		wantPage int
		wantSize int
	}{
		{name: "defaults when absent", query: "", wantPage: pagination.DefaultPage, wantSize: pagination.DefaultPageSize},
		{name: "explicit values", query: "?page=3&page_size=5", wantPage: 3, wantSize: 5},
		{name: "page_size clamped to max", query: "?page_size=9999", wantPage: 1, wantSize: pagination.MaxPageSize},
		{name: "page clamped to min", query: "?page=0", wantPage: pagination.MinPage, wantSize: pagination.DefaultPageSize},
		{name: "garbage falls back to defaults", query: "?page=abc&page_size=xyz", wantPage: pagination.DefaultPage, wantSize: pagination.DefaultPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got controllers.ListArticlesFilter
			ctrl := &mockArticleCtrl{
				listFn: func(_ context.Context, _ db.Querier, filter controllers.ListArticlesFilter) ([]db.Article, int64, error) {
					got = filter
					return []db.Article{}, 0, nil
				},
			}
			app := buildApp(ctrl, &mockArticleFeedCtrl{})

			resp := listArticles(t, app, tt.query, true)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.wantPage, got.Page.Page)
			assert.Equal(t, tt.wantSize, got.Page.PageSize)
		})
	}
}

// total_count comes from the controller's count, not from len(docs): a page holding 2 of 57 matches
// must still report 57, or the client cannot know there is more to fetch.
func TestListArticles_TotalCountComesFromController(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListArticlesFilter) ([]db.Article, int64, error) {
			return []db.Article{fixtures.NewTestArticle(), fixtures.NewTestArticleAlt()}, 57, nil
		},
	}
	app := buildApp(ctrl, &mockArticleFeedCtrl{})

	resp := listArticles(t, app, "?page=1&page_size=2", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env struct {
		Pagination pagination.Meta `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	assert.Equal(t, int64(57), env.Pagination.TotalCount)
	assert.Equal(t, 29, env.Pagination.TotalPages)
	assert.Equal(t, 2, env.Pagination.ActualCount)
	assert.True(t, env.Pagination.HasNextPage)
	assert.False(t, env.Pagination.HasPreviousPage)
}

func TestListArticles_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockArticleCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListArticlesFilter) ([]db.Article, int64, error) {
			return nil, 0, assert.AnError
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
