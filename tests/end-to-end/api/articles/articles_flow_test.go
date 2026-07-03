package articles_test

import (
	"context"
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
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	db "github.com/nathanap/news-feed-backend/sqlc"
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

// decodePage reads a paginated list response ({ docs, pagination }) and returns the docs slice.
func decodePage(t *testing.T, resp *http.Response) []map[string]any {
	t.Helper()
	var env struct {
		Docs []map[string]any `json:"docs"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs
}

func setupE2EApp(t *testing.T, oauth external.MockGoogleOAuth) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	prefCtrl := controllers.NewUserPreferencesController()
	userCtrl := controllers.NewUserController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authCtrl := controllers.NewAuthController(
		&oauth, userCtrl, refreshTokenCtrl, prefCtrl,
		runTx, []byte(jwtmock.TestJWTSecret), time.Hour,
	)
	articleCtrl := controllers.NewArticleController()
	afCtrl := controllers.NewArticleFeedController()
	sourceCtrl := controllers.NewSourceController()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	auth := app.Group("/v1/auth")
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))

	s := app.Group("/v1/sources")
	s.Post("/create", append(authMiddleware, sourceendpoints.CreateSource(sourceCtrl, runTx))...)

	a := app.Group("/v1/articles")
	a.Post("/create", append(authMiddleware, articleendpoints.CreateArticle(articleCtrl, runTx))...)
	a.Put("/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(articleCtrl, afCtrl, runTx))...)
	a.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, afCtrl, runTx))...)
	a.Get("", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, runTx))...)
	a.Put("/:id", append(authMiddleware, articleendpoints.UpdateArticle(articleCtrl, runTx))...)
	a.Delete("/:id", append(authMiddleware, articleendpoints.DeleteArticle(articleCtrl, runTx))...)

	return app, queries
}

// createSourceViaAPI creates a source through the real endpoint and returns its id.
// Articles require an active source; E2E seeds it through the API rather than the DB.
func createSourceViaAPI(t *testing.T, app *fiber.App, token, url, urlRss string) string {
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

func loginViaCallback(t *testing.T, app *fiber.App, queries db.Querier) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=any-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, readJSON(resp, &body))

	token, _ := body["access_token"].(string)
	require.NotEmpty(t, token)
	refreshTokenID, _ := body["refresh_token"].(string)
	require.NotEmpty(t, refreshTokenID)

	rt, err := queries.FindRefreshTokenByID(context.Background(), refreshTokenID)
	require.NoError(t, err)
	_, err = queries.FindUserByID(context.Background(), rt.UserID)
	require.NoError(t, err)

	return token
}

func TestE2E_Articles_FullCRUDFlow(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-articles-1", Email: "articles@example.com", Name: "Articles User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)
	sourceID := createSourceViaAPI(t, app, token, "https://e2e-source.com", "https://e2e-source.com/rss.xml")

	// Create
	createBody := `{"title":"E2E Article","content":"# E2E\n\nContent.","url_original":"https://e2e.com/a","keywords":["metallica","rock","metal","music","concert"],"source_id":"` + sourceID + `","language_original":"pt"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)
	assert.Equal(t, "E2E Article", created["title"])
	assert.Equal(t, sourceID, created["source_id"])
	assert.True(t, created["status"].(bool))

	// Get by ID
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	// List
	listReq, _ := http.NewRequest(http.MethodGet, "/v1/articles", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	listed := decodePage(t, listResp)
	assert.Len(t, listed, 1)

	// Update
	updateBody := `{"title":"E2E Updated","content":"# Updated","url_original":"https://e2e.com/a","keywords":["a","b","c","d","e"],"language_original":"pt"}`
	updateReq, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+id, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)
	var updated map[string]any
	require.NoError(t, readJSON(updateResp, &updated))
	assert.Equal(t, "E2E Updated", updated["title"])
	assert.Equal(t, sourceID, updated["source_id"], "source_id must be immutable across updates")

	// Delete
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/articles/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Gone
	afterReq, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+id, nil)
	afterReq.Header.Set("Authorization", "Bearer "+token)
	afterResp, err := app.Test(afterReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, afterResp.StatusCode)
}

func TestE2E_Articles_RequiresAuth(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/articles/create", `{"title":"T","content":"C","url_original":"https://x.com/a","keywords":["a","b","c","d","e"]}`},
		{http.MethodGet, "/v1/articles", ""},
		{http.MethodGet, "/v1/articles/some-id", ""},
		{http.MethodPut, "/v1/articles/some-id", `{"title":"T","content":"C","url_original":"https://x.com/a","keywords":["a","b","c","d","e"]}`},
		{http.MethodDelete, "/v1/articles/some-id", ""},
	} {
		req, err := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		require.NoError(t, err)
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "expected 401 for %s %s", tc.method, tc.path)
	}
}

func TestE2E_Articles_ValidationErrors(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-articles-2", Email: "val@example.com", Name: "Val User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)

	tt := []struct {
		name string
		body string
	}{
		{"missing title", `{"content":"C","url_original":"https://x.com/a","keywords":["a","b","c","d","e"]}`},
		{"missing content", `{"title":"T","url_original":"https://x.com/a","keywords":["a","b","c","d","e"]}`},
		{"invalid url", `{"title":"T","content":"C","url_original":"ftp://x.com","keywords":["a","b","c","d","e"]}`},
		{"too few keywords", `{"title":"T","content":"C","url_original":"https://x.com/a","keywords":["a","b"]}`},
		{"missing source_id", `{"title":"T","content":"C","url_original":"https://x.com/a","keywords":["a","b","c","d","e"]}`},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/v1/articles/create", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			readJSON(resp, &map[string]any{})
		})
	}
}
