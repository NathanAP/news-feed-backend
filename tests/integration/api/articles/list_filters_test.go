package articles_test

import (
	"database/sql"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/pagination"
)

// Filtering and pagination for GET /v1/articles happen in SQL, so they can only be proven against a
// real Postgres: these tests drive the endpoint over a live database and assert on the rows that
// actually come back. The unit tests next door only prove the handler forwards the right filter.

func listArticlesReq(t *testing.T, app interface {
	Test(*http.Request, ...fiber.TestConfig) (*http.Response, error)
}, token, query string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/articles"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return resp
}

// decodePageWithMeta reads both halves of the envelope: the docs and the pagination metadata.
func decodePageWithMeta(t *testing.T, resp *http.Response) ([]map[string]any, pagination.Meta) {
	t.Helper()
	var env struct {
		Docs       []map[string]any `json:"docs"`
		Pagination pagination.Meta  `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs, env.Pagination
}

// forceCreatedAt collapses every article onto one identical created_at, reproducing what a CRON
// batch does when it writes several articles within the same instant. The API has no way to set
// created_at, so this reaches past it into the database on purpose.
func forceCreatedAt(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.ExecContext(t.Context(), "UPDATE articles SET created_at = '2026-07-01T12:00:00Z'")
	require.NoError(t, err)
}

func TestIntegration_ListArticles_FilterByURL(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	createArticle(t, app, token, "https://example.com/news/one", sourceID)
	createArticle(t, app, token, "https://other-site.com/news/two", sourceID)

	docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, "?url=other-site"))
	require.Len(t, docs, 1)
	assert.Equal(t, "https://other-site.com/news/two", docs[0]["url_original"])
	assert.Equal(t, int64(1), meta.TotalCount, "total must count matches, not all rows")
}

// The filter is case-insensitive on both sides: the query is lowercased against a lowercased column.
func TestIntegration_ListArticles_FilterByURLIsCaseInsensitive(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	createArticle(t, app, token, "https://example.com/news/MiXeDcAsE", sourceID)

	docs, _ := decodePageWithMeta(t, listArticlesReq(t, app, token, "?url=mixedcase"))
	assert.Len(t, docs, 1)
}

// A url filter matching nothing is a 200 with an empty list, never a 404 (conventions.md: a search
// endpoint that finds nothing succeeded at searching).
func TestIntegration_ListArticles_FilterMatchingNothingIsEmpty200(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	createArticle(t, app, token, "https://example.com/news/one", sourceID)

	docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, "?url=nothing-matches-this"))
	assert.Empty(t, docs)
	assert.NotNil(t, docs)
	assert.Equal(t, int64(0), meta.TotalCount)
	assert.Equal(t, 0, meta.TotalPages)
}

// SQL wildcards must be matched literally. The query uses strpos rather than ILIKE '%...%' exactly
// so that a user's "%" or "_" is a character to find, not a pattern to expand. Both subtests below
// would return every row under an ILIKE implementation.
func TestIntegration_ListArticles_FilterTreatsWildcardsLiterally(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	// Neither URL contains a literal "%" or the sequence "_ews" — "%" cannot even appear unencoded
	// in a URL the create endpoint accepts, which is precisely why searching for one must find
	// nothing rather than everything.
	createArticle(t, app, token, "https://example.com/news/one", sourceID)
	createArticle(t, app, token, "https://example.com/news/two", sourceID)

	t.Run("percent finds nothing instead of matching all", func(t *testing.T) {
		docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, "?url=%25"))
		assert.Empty(t, docs, "a literal %% is absent from every url")
		assert.Equal(t, int64(0), meta.TotalCount)
	})

	t.Run("underscore does not stand for any single char", func(t *testing.T) {
		// "_ews" would match "news" in both rows if _ were a wildcard.
		docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, "?url=_ews"))
		assert.Empty(t, docs)
		assert.Equal(t, int64(0), meta.TotalCount)
	})
}

// The page/total_pages/out-of-range behaviour of the envelope is covered in pagination_test.go.

// The filter must be applied before the pagination, not to the page: filtering 6 rows down to 2 and
// asking for page 1 must yield those 2, not "whatever survived from the first page of all rows".
func TestIntegration_ListArticles_FilterAppliesBeforePagination(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	// 4 noise rows created first, then the 2 that match. Under a filter-after-paginate bug, page 1
	// would be filled by the noise and the matches would be missing.
	for i := 0; i < 4; i++ {
		createArticle(t, app, token, fmt.Sprintf("https://noise.com/news/%d", i), sourceID)
	}
	createArticle(t, app, token, "https://wanted.com/news/1", sourceID)
	createArticle(t, app, token, "https://wanted.com/news/2", sourceID)

	docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, "?url=wanted.com&page=1&page_size=10"))
	assert.Len(t, docs, 2)
	assert.Equal(t, int64(2), meta.TotalCount)
	assert.Equal(t, 1, meta.TotalPages)
}

// Soft-deleted rows must not count toward the total either — an inactive row leaking into
// total_count would make the client page toward rows that never arrive.
func TestIntegration_ListArticles_SoftDeletedExcludedFromTotal(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	createArticle(t, app, token, "https://example.com/news/kept", sourceID)
	deletedID := createArticle(t, app, token, "https://example.com/news/gone", sourceID)

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+deletedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, ""))
	assert.Len(t, docs, 1)
	assert.Equal(t, int64(1), meta.TotalCount)
}

// The ordering must be total, not just "newest first". Articles written in the same CRON batch share
// a created_at, and with LIMIT/OFFSET an unstable tie-break lets a row repeat on one page and vanish
// from another. id (a UUIDv7) is the tie-break; this walks every page and demands each row exactly
// once. Without the tie-break in the ORDER BY this test is what fails.
func TestIntegration_ListArticles_PaginationIsStableAcrossTiedTimestamps(t *testing.T) {
	requireNotProduction(t)

	app, queries, database := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	const total = 10
	created := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		id := createArticle(t, app, token, fmt.Sprintf("https://example.com/tied/%d", i), sourceID)
		created[id] = struct{}{}
	}
	// Force every row onto the same created_at, the way a discovery batch does. The API cannot
	// produce this state (created_at is CURRENT_TIMESTAMP), hence the raw UPDATE.
	forceCreatedAt(t, database)

	seen := make(map[string]int, total)
	for page := 1; page <= 5; page++ {
		docs, meta := decodePageWithMeta(t, listArticlesReq(t, app, token, fmt.Sprintf("?page=%d&page_size=2", page)))
		require.Equal(t, int64(total), meta.TotalCount)
		for _, doc := range docs {
			seen[doc["id"].(string)]++
		}
	}

	require.Len(t, seen, total, "every article must appear exactly once across the pages")
	for id := range created {
		assert.Equal(t, 1, seen[id], "article %s appeared %d times across pages", id, seen[id])
	}
}
