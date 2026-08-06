package sources_test

import (
	"context"
	"github.com/gofiber/fiber/v3"
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
// controller returns into the paginated envelope. That the filters really narrow rows is proven
// against a real Postgres in tests/integration/api/sources.

func listSources(t *testing.T, app interface {
	Test(*http.Request, ...fiber.TestConfig) (*http.Response, error)
}, query string, withAuth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/v1/sources"+query, nil)
	require.NoError(t, err)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestListSources_Success(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListSourcesFilter) ([]db.Source, int64, error) {
			return []db.Source{fixtures.NewTestSource(), fixtures.NewTestSourceAlt()}, 2, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	resp := listSources(t, app, "", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Len(t, result, 2)
}

func TestListSources_Empty(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListSourcesFilter) ([]db.Source, int64, error) {
			return []db.Source{}, 0, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	resp := listSources(t, app, "", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Empty(t, result)
}

// url and name are independent filters and must be forwarded independently: sending one must not
// populate the other (which would silently AND in a filter the client never asked for).
func TestListSources_ForwardsFilters(t *testing.T) {
	requireNotProduction(t)

	tests := []struct {
		name     string
		query    string
		wantURL  string
		wantName string
	}{
		{name: "no filters", query: "", wantURL: "", wantName: ""},
		{name: "url only", query: "?url=other-source", wantURL: "other-source", wantName: ""},
		{name: "name only", query: "?name=Example", wantURL: "", wantName: "Example"},
		{name: "both", query: "?url=other-source&name=Example", wantURL: "other-source", wantName: "Example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got controllers.ListSourcesFilter
			ctrl := &mockSourceCtrl{
				listFn: func(_ context.Context, _ db.Querier, filter controllers.ListSourcesFilter) ([]db.Source, int64, error) {
					got = filter
					return []db.Source{}, 0, nil
				},
			}
			app := buildApp(ctrl, http.DefaultClient)

			resp := listSources(t, app, tt.query, true)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.wantURL, got.URL)
			assert.Equal(t, tt.wantName, got.Name)
		})
	}
}

func TestListSources_ForwardsPaginationParams(t *testing.T) {
	requireNotProduction(t)

	var got controllers.ListSourcesFilter
	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier, filter controllers.ListSourcesFilter) ([]db.Source, int64, error) {
			got = filter
			return []db.Source{}, 0, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	resp := listSources(t, app, "?page=2&page_size=7", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, got.Page.Page)
	assert.Equal(t, 7, got.Page.PageSize)
}

// total_count must report every match, not the size of this page.
func TestListSources_TotalCountComesFromController(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListSourcesFilter) ([]db.Source, int64, error) {
			return []db.Source{fixtures.NewTestSource()}, 12, nil
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	resp := listSources(t, app, "?page=1&page_size=1", true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var env struct {
		Pagination pagination.Meta `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	assert.Equal(t, int64(12), env.Pagination.TotalCount)
	assert.Equal(t, 12, env.Pagination.TotalPages)
	assert.True(t, env.Pagination.HasNextPage)
}

func TestListSources_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSourceCtrl{
		listFn: func(_ context.Context, _ db.Querier, _ controllers.ListSourcesFilter) ([]db.Source, int64, error) {
			return nil, 0, assert.AnError
		},
	}
	app := buildApp(ctrl, http.DefaultClient)

	resp := listSources(t, app, "", true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestListSources_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := defaultApp()
	resp := listSources(t, app, "", false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
