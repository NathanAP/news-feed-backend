package articles_test

import (
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
	Test(*http.Request, ...int) (*http.Response, error)
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

	app, queries := setupIntegrationApp(t)
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
