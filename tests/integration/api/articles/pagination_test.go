package articles_test

import (
	"github.com/gofiber/fiber/v3"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// paginationMeta mirrors the pagination envelope for assertions.
type paginationMeta struct {
	ActualPage      int  `json:"actual_page"`
	TotalPages      int  `json:"total_pages"`
	ActualCount     int  `json:"actual_count"`
	TotalCount      int  `json:"total_count"`
	HasNextPage     bool `json:"has_next_page"`
	HasPreviousPage bool `json:"has_previous_page"`
}

func fetchArticlesPage(t *testing.T, app interface {
	Test(*http.Request, ...fiber.TestConfig) (*http.Response, error)
}, token, query string) (int, paginationMeta) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/articles"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var env struct {
		Docs       []map[string]any `json:"docs"`
		Pagination paginationMeta   `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	return len(env.Docs), env.Pagination
}

func TestIntegration_ListArticles_Pagination(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	// Create 5 articles.
	for _, u := range []string{"a", "b", "c", "d", "e"} {
		createArticle(t, app, token, "https://e.com/"+u, sourceID)
	}

	// Page 1 with page_size=2 → 2 docs, first of 3 pages.
	count, meta := fetchArticlesPage(t, app, token, "?page=1&page_size=2")
	assert.Equal(t, 2, count)
	assert.Equal(t, 1, meta.ActualPage)
	assert.Equal(t, 3, meta.TotalPages)
	assert.Equal(t, 5, meta.TotalCount)
	assert.Equal(t, 2, meta.ActualCount)
	assert.True(t, meta.HasNextPage)
	assert.False(t, meta.HasPreviousPage)

	// Last page → 1 doc, no next, has previous.
	count, meta = fetchArticlesPage(t, app, token, "?page=3&page_size=2")
	assert.Equal(t, 1, count)
	assert.False(t, meta.HasNextPage)
	assert.True(t, meta.HasPreviousPage)

	// Out-of-range page → empty docs, actual_page stays as requested, no error.
	count, meta = fetchArticlesPage(t, app, token, "?page=5&page_size=2")
	assert.Equal(t, 0, count)
	assert.Equal(t, 5, meta.ActualPage)
	assert.Equal(t, 3, meta.TotalPages)

	// No params → defaults (page 1, page_size 20) fit all 5 in one page.
	count, meta = fetchArticlesPage(t, app, token, "")
	assert.Equal(t, 5, count)
	assert.Equal(t, 1, meta.TotalPages)
	assert.False(t, meta.HasNextPage)
}

func TestIntegration_ListArticles_HugePageDoesNotOverflowOffset(t *testing.T) {
	requireNotProduction(t)

	// Regression (0.46.3): ParseParams clamped page from below but had no ceiling, and Offset() cast
	// (page-1)*page_size straight to int32. A page in the hundreds of millions wrapped the cast to a
	// NEGATIVE offset, which Postgres rejects outright ("OFFSET must not be negative"), turning a
	// user-supplied query param into a 500. An out-of-range page must behave like any other
	// out-of-range page: 200 with no docs, actual_page echoing the request.
	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)
	createArticle(t, app, token, "https://e.com/only", sourceID)

	for _, page := range []string{"107374183", "200000000", "9999999999"} {
		req, _ := http.NewRequest(http.MethodGet, "/v1/articles?page="+page+"&page_size=20", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "page=%s must not 500", page)

		var env struct {
			Docs       []map[string]any `json:"docs"`
			Pagination paginationMeta   `json:"pagination"`
		}
		require.NoError(t, readJSON(resp, &env))
		assert.Empty(t, env.Docs, "page=%s is far past the data", page)
		assert.Equal(t, 1, env.Pagination.TotalCount, "the total still reflects reality")
		assert.True(t, env.Pagination.HasPreviousPage)
		assert.False(t, env.Pagination.HasNextPage)
	}
}
