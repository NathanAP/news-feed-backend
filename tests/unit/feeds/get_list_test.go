package feeds_test

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

func TestGetFeed_Success(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/01900000-0000-7000-8000-000000000030", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Metallica Feed", result["name"])
}

func TestGetFeed_NotFound(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		findByIDFn: func(_ context.Context, _ db.Querier, _, _ string) (db.Feed, error) {
			return db.Feed{}, controllers.ErrFeedNotFound
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/other-user-feed", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetFeed_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/some-id", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListFeeds_Success(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, userID string, _ controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
			return []db.Feed{fixtures.NewTestFeed(userID), fixtures.NewTestFeedAlt(userID)}, 2, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Len(t, result, 2)
}

// Filtering and pagination run in SQL, so the handler owes an accurate filter: the name query must
// be forwarded, and the user scope must come from the token rather than from anything client-sent.
// That the filter really narrows rows is proven against a real Postgres in
// tests/integration/api/feeds.
func TestListFeeds_ForwardsFilterAndUserScope(t *testing.T) {
	requireNotProduction(t)

	tests := []struct {
		name     string
		query    string
		wantName string
	}{
		{name: "no filter", query: "", wantName: ""},
		{name: "name filter", query: "?name=Tech", wantName: "Tech"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got controllers.ListFeedsFilter
			var gotUserID string
			ctrl := &mockFeedCtrl{
				listFn: func(_ context.Context, _ db.Querier, userID string, filter controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
					got = filter
					gotUserID = userID
					return []db.Feed{}, 0, nil
				},
			}
			app := buildApp(ctrl)
			req, _ := http.NewRequest(http.MethodGet, "/v1/feeds"+tt.query, nil)
			req.Header.Set("Authorization", authHeader(t))
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			assert.Equal(t, tt.wantName, got.Name)
			assert.Equal(t, fixtures.NewTestUser().ID, gotUserID, "user scope must come from the token")
		})
	}
}

func TestListFeeds_ForwardsPaginationParams(t *testing.T) {
	requireNotProduction(t)

	var got controllers.ListFeedsFilter
	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ string, filter controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
			got = filter
			return []db.Feed{}, 0, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds?page=4&page_size=3", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 4, got.Page.Page)
	assert.Equal(t, 3, got.Page.PageSize)
}

// total_count must report every match, not the size of this page.
func TestListFeeds_TotalCountComesFromController(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, userID string, _ controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
			return []db.Feed{fixtures.NewTestFeed(userID)}, 5, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds?page=1&page_size=1", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var env struct {
		Pagination pagination.Meta `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	assert.Equal(t, int64(5), env.Pagination.TotalCount)
	assert.Equal(t, 5, env.Pagination.TotalPages)
}

func TestListFeeds_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ string, _ controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
			return nil, 0, assert.AnError
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestListFeeds_Empty(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockFeedCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ string, _ controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
			return []db.Feed{}, 0, nil
		},
	}
	app := buildApp(ctrl)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	req.Header.Set("Authorization", authHeader(t))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Empty(t, result)
}

func TestListFeeds_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
