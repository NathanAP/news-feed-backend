package sources_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/pagination"
)

// Filtering and pagination for GET /v1/sources happen in SQL, so they can only be proven against a
// real Postgres: these tests drive the endpoint over a live database and assert on the rows that
// actually come back. The unit tests only prove the handler forwards the right filter.

func seedNamedSource(t *testing.T, app *fiber.App, token, name, url, urlRss string) string {
	t.Helper()
	body := `{"name":"` + name + `","url":"` + url + `","url_rss":"` + urlRss + `"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	require.NoError(t, readJSON(resp, &created))
	return created["id"].(string)
}

func listSourcesReq(t *testing.T, app *fiber.App, token, query string) ([]map[string]any, pagination.Meta) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/sources"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var env struct {
		Docs       []map[string]any `json:"docs"`
		Pagination pagination.Meta  `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs, env.Pagination
}

// seedThreeSources sets up rows where name and url deliberately do not correlate, so a test can tell
// which filter actually ran.
func seedThreeSources(t *testing.T, app *fiber.App, token string) {
	t.Helper()
	seedNamedSource(t, app, token, "Tech Daily", "https://alpha.com", "https://alpha.com/rss.xml")
	seedNamedSource(t, app, token, "Sports Weekly", "https://beta.com", "https://beta.com/rss.xml")
	seedNamedSource(t, app, token, "Tech Review", "https://gamma.com", "https://gamma.com/rss.xml")
}

func TestIntegration_ListSources_FilterByURL(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)
	seedThreeSources(t, app, token)

	docs, meta := listSourcesReq(t, app, token, "?url=beta")
	require.Len(t, docs, 1)
	assert.Equal(t, "https://beta.com", docs[0]["url"])
	assert.Equal(t, int64(1), meta.TotalCount)
}

// The name filter is new in 0.38: sources have had a name since 0.30, but it was never searchable.
func TestIntegration_ListSources_FilterByName(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)
	seedThreeSources(t, app, token)

	docs, meta := listSourcesReq(t, app, token, "?name=Tech")
	require.Len(t, docs, 2, "both Tech sources match on a substring")
	assert.Equal(t, int64(2), meta.TotalCount)
	for _, doc := range docs {
		assert.Contains(t, doc["name"], "Tech")
	}
}

func TestIntegration_ListSources_FilterByNameIsCaseInsensitive(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)
	seedThreeSources(t, app, token)

	docs, _ := listSourcesReq(t, app, token, "?name=tECH")
	assert.Len(t, docs, 2)
}

// url and name are ANDed, not ORed: passing both must narrow to the rows satisfying each one. The
// data is arranged so an OR would return 2 and only an AND returns 1.
func TestIntegration_ListSources_FiltersCombineWithAnd(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)
	seedThreeSources(t, app, token)

	docs, meta := listSourcesReq(t, app, token, "?name=Tech&url=gamma")
	require.Len(t, docs, 1)
	assert.Equal(t, "Tech Review", docs[0]["name"])
	assert.Equal(t, "https://gamma.com", docs[0]["url"])
	assert.Equal(t, int64(1), meta.TotalCount)
}

// A combination no row satisfies is a 200 with an empty list, never a 404.
func TestIntegration_ListSources_FiltersMatchingNothingIsEmpty200(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)
	seedThreeSources(t, app, token)

	// "Sports" exists and "gamma" exists, but not on the same row.
	docs, meta := listSourcesReq(t, app, token, "?name=Sports&url=gamma")
	assert.Empty(t, docs)
	assert.NotNil(t, docs)
	assert.Equal(t, int64(0), meta.TotalCount)
	assert.Equal(t, 0, meta.TotalPages)
}

func TestIntegration_ListSources_Paginates(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	const total = 5
	for i := 0; i < total; i++ {
		seedNamedSource(t, app, token,
			fmt.Sprintf("Source %d", i),
			fmt.Sprintf("https://s%d.com", i),
			fmt.Sprintf("https://s%d.com/rss.xml", i))
	}

	docs, meta := listSourcesReq(t, app, token, "?page=1&page_size=2")
	assert.Len(t, docs, 2)
	assert.Equal(t, 3, meta.TotalPages)
	assert.Equal(t, int64(total), meta.TotalCount)
	assert.True(t, meta.HasNextPage)

	// A page past the end: empty, but the metadata still describes reality (PROJECT.md).
	docs, meta = listSourcesReq(t, app, token, "?page=9&page_size=2")
	assert.Empty(t, docs)
	assert.Equal(t, 9, meta.ActualPage)
	assert.Equal(t, 3, meta.TotalPages)
	assert.Equal(t, int64(total), meta.TotalCount)
}

// Filtering must happen before pagination: the total and the page must both describe the filtered
// set, not the whole table.
func TestIntegration_ListSources_FilterAppliesBeforePagination(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	for i := 0; i < 4; i++ {
		seedNamedSource(t, app, token,
			fmt.Sprintf("Noise %d", i),
			fmt.Sprintf("https://noise%d.com", i),
			fmt.Sprintf("https://noise%d.com/rss.xml", i))
	}
	seedNamedSource(t, app, token, "Wanted One", "https://wanted1.com", "https://wanted1.com/rss.xml")
	seedNamedSource(t, app, token, "Wanted Two", "https://wanted2.com", "https://wanted2.com/rss.xml")

	docs, meta := listSourcesReq(t, app, token, "?name=Wanted&page=1&page_size=10")
	assert.Len(t, docs, 2)
	assert.Equal(t, int64(2), meta.TotalCount)
	assert.Equal(t, 1, meta.TotalPages)
}

// A soft-deleted source must not survive a filter either: status is never an exposed filter, and an
// inactive row must be invisible to every query path (PROJECT.md).
func TestIntegration_ListSources_SoftDeletedExcludedFromFilterAndTotal(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	seedNamedSource(t, app, token, "Tech Daily", "https://alpha.com", "https://alpha.com/rss.xml")
	deletedID := seedNamedSource(t, app, token, "Tech Review", "https://gamma.com", "https://gamma.com/rss.xml")

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/sources/"+deletedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	docs, meta := listSourcesReq(t, app, token, "?name=Tech")
	require.Len(t, docs, 1)
	assert.Equal(t, "Tech Daily", docs[0]["name"])
	assert.Equal(t, int64(1), meta.TotalCount)
}
