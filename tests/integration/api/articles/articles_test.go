package articles_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
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

// decodePage reads a paginated list response ({ docs, pagination }) and returns the docs slice.
func decodePage(t *testing.T, resp *http.Response) []map[string]any {
	t.Helper()
	var env struct {
		Docs []map[string]any `json:"docs"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs
}

// setupIntegrationApp wires the articles endpoints over a real, throwaway Postgres. The raw *sql.DB
// comes back alongside the Querier because a few tests need to set up state the API deliberately
// cannot produce — see forceCreatedAt.
func setupIntegrationApp(t *testing.T) (*fiber.App, db.Querier, *sql.DB) {
	return setupIntegrationAppClient(t, "")
}

// setupIntegrationAppClient is setupIntegrationApp with a configurable client URL, so the read-time
// outbound-link swap (which expands the {CLIENT_URL} token) can be exercised end to end.
func setupIntegrationAppClient(t *testing.T, clientURL string) (*fiber.App, db.Querier, *sql.DB) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	articleCtrl := controllers.NewArticleController()
	outboundCtrl := controllers.NewArticleOutboundLinkController()
	afCtrl := controllers.NewArticleFeedController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New()
	a := app.Group("/v1/articles")
	testutils.AddRoute(a, fiber.MethodPost, "/create", append(authMiddleware, articleendpoints.CreateArticle(articleCtrl, runTx)))
	testutils.AddRoute(a, fiber.MethodPut, "/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(articleCtrl, afCtrl, runTx)))
	testutils.AddRoute(a, fiber.MethodGet, "/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, afCtrl, outboundCtrl, runTx, clientURL)))
	testutils.AddRoute(a, fiber.MethodGet, "", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, outboundCtrl, runTx, clientURL)))
	testutils.AddRoute(a, fiber.MethodPut, "/:id", append(authMiddleware, articleendpoints.UpdateArticle(articleCtrl, runTx)))
	testutils.AddRoute(a, fiber.MethodDelete, "/:id", append(authMiddleware, articleendpoints.DeleteArticle(articleCtrl, outboundCtrl, runTx)))

	return app, queries, database
}

func seedUser(t *testing.T, queries db.Querier) string {
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
	return token
}

// seedSource inserts the standard test source directly in the DB and returns its id.
// Articles require an existing active source, so tests seed one before creating articles.
func seedSource(t *testing.T, queries db.Querier) string {
	t.Helper()
	source := fixtures.NewTestSource()
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID:     source.ID,
		Name:   source.Name,
		Url:    source.Url,
		UrlRss: source.UrlRss,
	})
	require.NoError(t, err)
	return source.ID
}

func createArticle(t *testing.T, app *fiber.App, token, urlOriginal, sourceID string) string {
	t.Helper()
	body := `{"title":"T","content":"# C","url_original":"` + urlOriginal + `","keywords":["a","b","c","d","e"],"source_id":"` + sourceID + `","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]any
	require.NoError(t, readJSON(resp, &created))
	return created["id"].(string)
}

// TestIntegration_GetArticle_ResolvesOutboundLinks proves the read path swaps outbound-link ids in the
// stored body back to real URLs, expanding the {CLIENT_URL} token to the configured client URL.
func TestIntegration_GetArticle_ResolvesOutboundLinks(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationAppClient(t, "https://client.app")
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	const articleID = "01900000-0000-7000-8000-0000000000a1"
	_, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID:          articleID,
		Title:       "Body with links",
		Content:     `<p><a href="oid-int">internal</a> and <a href="oid-ext">external</a></p>`,
		UrlOriginal: "https://src.com/body",
		Keywords:    json.RawMessage(`["a","b","c","d","e"]`),
		SourceID:    sourceID,
	})
	require.NoError(t, err)
	// Outbound rows: an internal link stored as the token, an external one stored literally.
	_, err = queries.CreateArticleOutboundLink(t.Context(), db.CreateArticleOutboundLinkParams{ID: "oid-int", ArticleID: articleID, Href: "{CLIENT_URL}/articles/target-123"})
	require.NoError(t, err)
	_, err = queries.CreateArticleOutboundLink(t.Context(), db.CreateArticleOutboundLinkParams{ID: "oid-ext", ArticleID: articleID, Href: "https://ext.com/z"})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+articleID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]any
	require.NoError(t, readJSON(resp, &got))
	content := got["content"].(string)
	assert.Contains(t, content, `href="https://client.app/articles/target-123"`, "internal token expanded to the client URL")
	assert.Contains(t, content, `href="https://ext.com/z"`, "external link restored literally")
	assert.NotContains(t, content, `href="oid-`, "no outbound id leaks to the client")
}

// ── Create ───────────────────────────────────────────────────────────────────

func TestIntegration_CreateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	body := `{"title":"Hello","content":"# Hello","url_original":"https://e.com/hello","keywords":["a","b","c","d","e"],"source_id":"` + sourceID + `","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
	assert.Equal(t, sourceID, result["source_id"])
	keywords := result["keywords"].([]any)
	assert.Len(t, keywords, 5)
}

func TestIntegration_CreateArticle_DuplicateURL(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	createArticle(t, app, token, "https://e.com/dup", sourceID)

	body := `{"title":"T","content":"# C","url_original":"https://e.com/dup","keywords":["a","b","c","d","e"],"source_id":"` + sourceID + `","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestIntegration_CreateArticle_KeywordsPersistedAndReturned(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	body := `{"title":"T","content":"# C","url_original":"https://e.com/kw","keywords":["metallica","rock","metal","music","concert"],"source_id":"` + sourceID + `","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created map[string]any
	require.NoError(t, readJSON(resp, &created))
	id := created["id"].(string)

	// Fetch back and verify keywords round-trip through JSON TEXT storage
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetched map[string]any
	require.NoError(t, readJSON(getResp, &fetched))
	keywords := fetched["keywords"].([]any)
	require.Len(t, keywords, 5)
	assert.Equal(t, "metallica", keywords[0])
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestIntegration_GetArticle_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/non-existent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestIntegration_ListArticles_ReturnsAll(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	createArticle(t, app, token, "https://a.com/1", sourceID)
	createArticle(t, app, token, "https://b.com/2", sourceID)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := decodePage(t, resp)
	assert.Len(t, result, 2)
}

func TestIntegration_ListArticles_ExcludesSoftDeleted(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	keptID := createArticle(t, app, token, "https://kept.com/1", sourceID)
	deletedID := createArticle(t, app, token, "https://gone.com/2", sourceID)

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+deletedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	listReq, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)

	result := decodePage(t, listResp)
	require.Len(t, result, 1)
	assert.Equal(t, keptID, result[0]["id"])
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestIntegration_UpdateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	id := createArticle(t, app, token, "https://e.com/upd", sourceID)

	body := `{"title":"Updated","content":"# New","url_original":"https://e.com/upd","keywords":["x","y","z","w","v"],"language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Updated", result["title"])
	assert.NotNil(t, result["modified_at"])
}

func TestIntegration_UpdateArticle_NotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)

	body := `{"title":"T","content":"# C","url_original":"https://e.com/x","keywords":["a","b","c","d","e"],"language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/articles/non-existent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestIntegration_DeleteArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)

	sourceID := seedSource(t, queries)
	id := createArticle(t, app, token, "https://e.com/del", sourceID)

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	getReq, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func TestIntegration_DeleteArticle_RetargetsInboundOutboundLinks(t *testing.T) {
	requireNotProduction(t)

	// 0.46.1: the "pre-remove cleanup" the 0.45 query comment promised was never wired — only the
	// retroactive-linking caller of Retarget existed. Without it, an older article keeps an anchor to
	// the removed article's internal page, which answers 404 because reads exclude removed articles.
	const clientURL = "https://client.app"
	app, queries, _ := setupIntegrationAppClient(t, clientURL)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	const doomedURL = "https://e.com/doomed"
	doomedID := createArticle(t, app, token, doomedURL, sourceID)

	// An older article links the doomed one internally: the row holds the {CLIENT_URL} token form.
	linkID, err := uuid.NewV7()
	require.NoError(t, err)
	linkerID := createArticle(t, app, token, "https://e.com/linker", sourceID)
	_, err = queries.CreateArticleOutboundLink(t.Context(), db.CreateArticleOutboundLinkParams{
		ID:        linkID.String(),
		ArticleID: linkerID,
		Href:      outboundlinks.InternalHref(doomedID),
	})
	require.NoError(t, err)

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+doomedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	links, err := queries.ListArticleOutboundLinksByArticleIDs(t.Context(), []string{linkerID})
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, doomedURL, links[0].Href,
		"the link must fall back to the source URL, not keep pointing at a page that now 404s")
	assert.NotContains(t, links[0].Href, "{CLIENT_URL}")
}

func TestIntegration_DeleteArticle_LeavesUnrelatedOutboundLinksAlone(t *testing.T) {
	requireNotProduction(t)

	// The retarget matches by exact href, so removing one article must not disturb links pointing
	// anywhere else — including internal links to a *different* article.
	const clientURL = "https://client.app"
	app, queries, _ := setupIntegrationAppClient(t, clientURL)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	doomedID := createArticle(t, app, token, "https://e.com/doomed2", sourceID)
	otherID := createArticle(t, app, token, "https://e.com/other", sourceID)
	linkerID := createArticle(t, app, token, "https://e.com/linker2", sourceID)

	externalLinkID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = queries.CreateArticleOutboundLink(t.Context(), db.CreateArticleOutboundLinkParams{
		ID: externalLinkID.String(), ArticleID: linkerID, Href: "https://external.com/keep",
	})
	require.NoError(t, err)

	otherLinkID, err := uuid.NewV7()
	require.NoError(t, err)
	_, err = queries.CreateArticleOutboundLink(t.Context(), db.CreateArticleOutboundLinkParams{
		ID: otherLinkID.String(), ArticleID: linkerID, Href: outboundlinks.InternalHref(otherID),
	})
	require.NoError(t, err)

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+doomedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	links, err := queries.ListArticleOutboundLinksByArticleIDs(t.Context(), []string{linkerID})
	require.NoError(t, err)
	byID := make(map[string]string, len(links))
	for _, l := range links {
		byID[l.ID] = l.Href
	}
	assert.Equal(t, "https://external.com/keep", byID[externalLinkID.String()], "external link untouched")
	assert.Equal(t, outboundlinks.InternalHref(otherID), byID[otherLinkID.String()],
		"internal link to a different article untouched")
}

func TestIntegration_CreateArticle_AfterSoftDelete(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	id := createArticle(t, app, token, "https://e.com/reused", sourceID)

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Re-create with same url_original — partial unique index must allow it
	body := `{"title":"T","content":"# C","url_original":"https://e.com/reused","keywords":["a","b","c","d","e"],"source_id":"` + sourceID + `","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// ── source_id rules ────────────────────────────────────────────────────────────

func TestIntegration_CreateArticle_SourceNotFound(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	// No source seeded — the referenced source does not exist.

	body := `{"title":"T","content":"# C","url_original":"https://e.com/ns","keywords":["a","b","c","d","e"],"source_id":"01900000-0000-7000-8000-000000000099","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_CreateArticle_SourceInactive(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	// Soft-delete the source: it must no longer be assignable to a new article.
	require.NoError(t, queries.SoftDeleteSource(t.Context(), sourceID))

	body := `{"title":"T","content":"# C","url_original":"https://e.com/inactive","keywords":["a","b","c","d","e"],"source_id":"` + sourceID + `","language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_UpdateArticle_SourceIDImmutable(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUser(t, queries)
	sourceID := seedSource(t, queries)

	id := createArticle(t, app, token, "https://e.com/imm", sourceID)

	// Update payload carries no source_id; the original must be preserved.
	body := `{"title":"Changed","content":"# Changed","url_original":"https://e.com/imm","keywords":["a","b","c","d","e"],"language_original":"pt"}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Changed", result["title"])
	assert.Equal(t, sourceID, result["source_id"], "source_id must remain unchanged after update")
}

func TestIntegration_Articles_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
