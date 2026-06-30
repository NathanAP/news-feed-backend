package sources_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	_ "modernc.org/sqlite"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func setupIntegrationApp(t *testing.T, httpClient *http.Client) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	sourceCtrl := controllers.NewSourceController()
	articleCtrl := controllers.NewArticleController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	s := app.Group("/v1/sources")
	s.Post("/create", append(authMiddleware, sourceendpoints.CreateSource(sourceCtrl, runTx))...)
	s.Get("/rss-discovery", append(authMiddleware, sourceendpoints.RSSDiscovery(httpClient))...)
	s.Get("/:id/article-discovery", append(authMiddleware, sourceendpoints.SourceArticleDiscovery(sourceCtrl, articleCtrl, runTx, httpClient))...)
	s.Get("/:id", append(authMiddleware, sourceendpoints.GetSource(sourceCtrl, runTx))...)
	s.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)
	s.Put("/:id", append(authMiddleware, sourceendpoints.UpdateSource(sourceCtrl, runTx))...)
	s.Delete("/:id", append(authMiddleware, sourceendpoints.DeleteSource(sourceCtrl, runTx))...)

	return app, queries
}

func seedUser(t *testing.T, queries db.Querier) (db.User, string) {
	t.Helper()
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	rt := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID:        rt.ID,
		UserID:    rt.UserID,
		ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)

	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return user, token
}

// ── Create ───────────────────────────────────────────────────────────────────

func TestIntegration_CreateSource_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	body := `{"url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
}

func TestIntegration_CreateSource_DuplicateURL(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	body := `{"url":"https://dup.com","url_rss":"https://dup.com/rss.xml"}`

	req1, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp1.StatusCode)
	readJSON(resp1, &map[string]any{})

	req2, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp2.StatusCode)
}

// TestIntegration_CreateSource_AfterSoftDelete enforces the status convention at the
// uniqueness level: a soft-deleted source must not block creating a new source with the
// same url/url_rss. Regression test for the partial-unique-index fix (0.12.1.0).
func TestIntegration_CreateSource_AfterSoftDelete(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	body := `{"url":"https://reused.com","url_rss":"https://reused.com/rss.xml"}`

	// Create the source
	create1, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	create1.Header.Set("Content-Type", "application/json")
	create1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := app.Test(create1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	var created map[string]any
	require.NoError(t, readJSON(resp1, &created))
	firstID := created["id"].(string)

	// Soft-delete it
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/sources/"+firstID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Re-create a source with the same url/url_rss — must succeed (not 409)
	create2, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	create2.Header.Set("Content-Type", "application/json")
	create2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(create2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp2.StatusCode)

	var recreated map[string]any
	require.NoError(t, readJSON(resp2, &recreated))
	assert.NotEqual(t, firstID, recreated["id"], "re-created source should be a new record")

	// The list must contain only the new active source
	listReq, _ := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	var listed []map[string]any
	require.NoError(t, readJSON(listResp, &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, recreated["id"], listed[0]["id"])
}

func TestIntegration_CreateSource_DuplicateActiveStillRejected(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	body := `{"url":"https://active-dup.com","url_rss":"https://active-dup.com/rss.xml"}`

	create1, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	create1.Header.Set("Content-Type", "application/json")
	create1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := app.Test(create1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp1.StatusCode)
	readJSON(resp1, &map[string]any{})

	// Second active source with same url — partial unique index must still reject it
	create2, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	create2.Header.Set("Content-Type", "application/json")
	create2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(create2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp2.StatusCode)
}

func TestIntegration_CreateSource_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupIntegrationApp(t, http.DefaultClient)
	body := `{"url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestIntegration_GetSource_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	// Create a source first
	createBody := `{"url":"https://get-test.com","url_rss":"https://get-test.com/rss.xml"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/"+id, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, id, result["id"])
	assert.Equal(t, "https://get-test.com", result["url"])
}

func TestIntegration_GetSource_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/non-existent-id", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestIntegration_ListSources_ReturnsAll(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	for i, pair := range [][]string{
		{"https://a.com", "https://a.com/rss.xml"},
		{"https://b.com", "https://b.com/feed.xml"},
	} {
		b := strings.NewReader(`{"url":"` + pair[0] + `","url_rss":"` + pair[1] + `"}`)
		r, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", b)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(r)
		require.NoError(t, err, "seed source %d", i)
		readJSON(resp, &map[string]any{})
	}

	req, err := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result, 2)
}

func TestIntegration_ListSources_EmptyDB(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Empty(t, result)
}

// TestIntegration_ListSources_ExcludesSoftDeleted enforces the status convention:
// a soft-deleted source (status = 0, removed_at set) must never appear in lists.
func TestIntegration_ListSources_ExcludesSoftDeleted(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	createSource := func(url, urlRss string) string {
		b := strings.NewReader(`{"url":"` + url + `","url_rss":"` + urlRss + `"}`)
		r, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", b)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(r)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var created map[string]any
		require.NoError(t, readJSON(resp, &created))
		return created["id"].(string)
	}

	keptID := createSource("https://kept.com", "https://kept.com/rss.xml")
	deletedID := createSource("https://gone.com", "https://gone.com/rss.xml")

	// Soft-delete the second source
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/sources/"+deletedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// List must contain only the active source
	listReq, _ := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(listResp, &result))
	require.Len(t, result, 1)
	assert.Equal(t, keptID, result[0]["id"])
	assert.NotEqual(t, deletedID, result[0]["id"])
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestIntegration_UpdateSource_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	createBody := `{"url":"https://upd.com","url_rss":"https://upd.com/rss.xml"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)

	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)

	updateBody := `{"url":"https://updated.com","url_rss":"https://updated.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/"+id, strings.NewReader(updateBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "https://updated.com", result["url"])
}

func TestIntegration_UpdateSource_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	body := `{"url":"https://example.com","url_rss":"https://example.com/rss.xml"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/sources/non-existent", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestIntegration_DeleteSource_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	createBody := `{"url":"https://del.com","url_rss":"https://del.com/rss.xml"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)

	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)

	delReq, err := http.NewRequest(http.MethodDelete, "/v1/sources/"+id, nil)
	require.NoError(t, err)
	delReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Source should no longer be retrievable
	getReq, err := http.NewRequest(http.MethodGet, "/v1/sources/"+id, nil)
	require.NoError(t, err)
	getReq.Header.Set("Authorization", "Bearer "+token)

	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func TestIntegration_DeleteSource_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)

	req, err := http.NewRequest(http.MethodDelete, "/v1/sources/non-existent", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── RSS Discovery ─────────────────────────────────────────────────────────────

func TestIntegration_RSSDiscovery_FindsFeed(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://mock-site.com": {
			StatusCode: http.StatusOK,
			Body:       external.SampleHTMLWithRSSLink,
		},
		"https://mock-site.com/rss.xml": {
			StatusCode: http.StatusOK,
			Body:       external.SampleRSSFeed,
		},
	})
	app, queries := setupIntegrationApp(t, mockClient)
	_, token := seedUser(t, queries)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss-discovery?url=https://mock-site.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds := result["feeds"].([]any)
	assert.Len(t, feeds, 1)
}

func TestIntegration_RSSDiscovery_EmptyOnNoFeeds(t *testing.T) {
	requireNotProduction(t)

	mockClient := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://no-feed.com": {
			StatusCode: http.StatusOK,
			Body:       external.SampleHTMLNoFeed,
		},
	})
	app, queries := setupIntegrationApp(t, mockClient)
	_, token := seedUser(t, queries)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss-discovery?url=https://no-feed.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds := result["feeds"].([]any)
	assert.Empty(t, feeds)
}

// ── Cascade source → articles ──────────────────────────────────────────────────

// seedArticleForSource inserts an article tied to sourceID directly in the DB.
func seedArticleForSource(t *testing.T, queries db.Querier, id, urlOriginal, sourceID string) {
	t.Helper()
	_, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID:          id,
		Title:       "Cascade Article",
		Content:     "# Cascade",
		UrlOriginal: urlOriginal,
		Keywords:    `["a","b","c","d","e"]`,
		SourceID:    sourceID,
	})
	require.NoError(t, err)
}

// TestIntegration_DeleteSource_CascadesToArticles enforces the cascade rule: soft-deleting
// a source must soft-delete every article that belongs to it, leaving other sources' articles
// untouched.
func TestIntegration_DeleteSource_CascadesToArticles(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t, http.DefaultClient)
	_, token := seedUser(t, queries)
	articleCtrl := controllers.NewArticleController()

	// Two sources created through the API.
	sourceA := createSourceReturningID(t, app, token, "https://a.com", "https://a.com/rss")
	sourceB := createSourceReturningID(t, app, token, "https://b.com", "https://b.com/rss")

	seedArticleForSource(t, queries, "01900000-0000-7000-8000-0000000000a1", "https://news.com/a1", sourceA)
	seedArticleForSource(t, queries, "01900000-0000-7000-8000-0000000000a2", "https://news.com/a2", sourceA)
	seedArticleForSource(t, queries, "01900000-0000-7000-8000-0000000000b1", "https://news.com/b1", sourceB)

	// Delete source A through the endpoint (which runs the cascade in its transaction).
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/sources/"+sourceA, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Source A's articles are now soft-deleted (no longer findable).
	_, err = articleCtrl.FindByID(t.Context(), queries, "01900000-0000-7000-8000-0000000000a1")
	assert.ErrorIs(t, err, controllers.ErrArticleNotFound)
	_, err = articleCtrl.FindByID(t.Context(), queries, "01900000-0000-7000-8000-0000000000a2")
	assert.ErrorIs(t, err, controllers.ErrArticleNotFound)

	// Source B's article remains active.
	b1, err := articleCtrl.FindByID(t.Context(), queries, "01900000-0000-7000-8000-0000000000b1")
	require.NoError(t, err)
	assert.Equal(t, sourceB, b1.SourceID)
}

// createSourceReturningID creates a source through the API and returns its id.
func createSourceReturningID(t *testing.T, app *fiber.App, token, url, urlRss string) string {
	t.Helper()
	body := `{"url":"` + url + `","url_rss":"` + urlRss + `"}`
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
