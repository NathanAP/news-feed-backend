package articles_test

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
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
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

func setupIntegrationApp(t *testing.T) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	articleCtrl := controllers.NewArticleController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	a := app.Group("/v1/articles")
	a.Post("/create", append(authMiddleware, articleendpoints.CreateArticle(articleCtrl, runTx))...)
	a.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, runTx))...)
	a.Get("", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, runTx))...)
	a.Put("/:id", append(authMiddleware, articleendpoints.UpdateArticle(articleCtrl, runTx))...)
	a.Delete("/:id", append(authMiddleware, articleendpoints.DeleteArticle(articleCtrl, runTx))...)

	return app, queries
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

func createArticle(t *testing.T, app *fiber.App, token, urlOriginal string) string {
	t.Helper()
	body := `{"title":"T","content":"# C","url_original":"` + urlOriginal + `","keywords":["a","b","c","d","e"]}`
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

// ── Create ───────────────────────────────────────────────────────────────────

func TestIntegration_CreateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	body := `{"title":"Hello","content":"# Hello","url_original":"https://e.com/hello","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
	keywords := result["keywords"].([]any)
	assert.Len(t, keywords, 5)
}

func TestIntegration_CreateArticle_DuplicateURL(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	createArticle(t, app, token, "https://e.com/dup")

	body := `{"title":"T","content":"# C","url_original":"https://e.com/dup","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestIntegration_CreateArticle_KeywordsPersistedAndReturned(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	body := `{"title":"T","content":"# C","url_original":"https://e.com/kw","keywords":["metallica","rock","metal","music","concert"]}`
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

	app, queries := setupIntegrationApp(t)
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

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	createArticle(t, app, token, "https://a.com/1")
	createArticle(t, app, token, "https://b.com/2")

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Len(t, result, 2)
}

func TestIntegration_ListArticles_ExcludesSoftDeleted(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	keptID := createArticle(t, app, token, "https://kept.com/1")
	deletedID := createArticle(t, app, token, "https://gone.com/2")

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+deletedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	listReq, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)

	var result []map[string]any
	require.NoError(t, readJSON(listResp, &result))
	require.Len(t, result, 1)
	assert.Equal(t, keptID, result[0]["id"])
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestIntegration_UpdateArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	id := createArticle(t, app, token, "https://e.com/upd")

	body := `{"title":"Updated","content":"# New","url_original":"https://e.com/upd","keywords":["x","y","z","w","v"]}`
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

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	body := `{"title":"T","content":"# C","url_original":"https://e.com/x","keywords":["a","b","c","d","e"]}`
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

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	id := createArticle(t, app, token, "https://e.com/del")

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

func TestIntegration_CreateArticle_AfterSoftDelete(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUser(t, queries)

	id := createArticle(t, app, token, "https://e.com/reused")

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Re-create with same url_original — partial unique index must allow it
	body := `{"title":"T","content":"# C","url_original":"https://e.com/reused","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestIntegration_Articles_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupIntegrationApp(t)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
