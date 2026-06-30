package sources_test

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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
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

func setupE2EApp(t *testing.T, oauth external.MockGoogleOAuth, httpClient *http.Client) (*fiber.App, db.Querier) {
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
	sourceCtrl := controllers.NewSourceController()
	systemCtrl := controllers.NewSystemController()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	auth := app.Group("/v1/auth")
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))

	s := app.Group("/v1/sources")
	s.Post("/create", append(authMiddleware, sourceendpoints.CreateSource(sourceCtrl, runTx))...)
	s.Get("/rss_discovery", append(authMiddleware, sourceendpoints.RSSDiscovery(httpClient))...)
	s.Get("/:id/discovery", append(authMiddleware, sourceendpoints.SourceDiscovery(sourceCtrl, systemCtrl, runTx, httpClient))...)
	s.Get("/:id", append(authMiddleware, sourceendpoints.GetSource(sourceCtrl, runTx))...)
	s.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)
	s.Put("/:id", append(authMiddleware, sourceendpoints.UpdateSource(sourceCtrl, runTx))...)
	s.Delete("/:id", append(authMiddleware, sourceendpoints.DeleteSource(sourceCtrl, runTx))...)

	return app, queries
}

func testOAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:3000/v1/auth/google/callback",
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func loginViaCallback(t *testing.T, app *fiber.App, queries db.Querier) (accessToken string) {
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

// ── Full CRUD flow ────────────────────────────────────────────────────────────

func TestE2E_Sources_FullCRUDFlow(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-sources-1", Email: "sources@example.com", Name: "Sources User",
		},
	}
	app, queries := setupE2EApp(t, oauth, http.DefaultClient)
	token := loginViaCallback(t, app, queries)

	// Create
	createBody := `{"url":"https://e2e-source.com","url_rss":"https://e2e-source.com/rss.xml"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)
	assert.Equal(t, "https://e2e-source.com", created["url"])
	assert.True(t, created["status"].(bool))

	// Get by ID
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetched map[string]any
	require.NoError(t, readJSON(getResp, &fetched))
	assert.Equal(t, id, fetched["id"])

	// List
	listReq, _ := http.NewRequest(http.MethodGet, "/v1/sources", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	var listed []map[string]any
	require.NoError(t, readJSON(listResp, &listed))
	assert.Len(t, listed, 1)

	// Update
	updateBody := `{"url":"https://e2e-updated.com","url_rss":"https://e2e-updated.com/feed.xml"}`
	updateReq, _ := http.NewRequest(http.MethodPut, "/v1/sources/"+id, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	var updated map[string]any
	require.NoError(t, readJSON(updateResp, &updated))
	assert.Equal(t, "https://e2e-updated.com", updated["url"])
	assert.NotNil(t, updated["modified_at"])

	// Delete
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/sources/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Deleted source is no longer available
	getAfterDel, _ := http.NewRequest(http.MethodGet, "/v1/sources/"+id, nil)
	getAfterDel.Header.Set("Authorization", "Bearer "+token)
	afterDelResp, err := app.Test(getAfterDel)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, afterDelResp.StatusCode)
}

// ── Uniqueness constraint ─────────────────────────────────────────────────────

func TestE2E_Sources_DuplicateURLRejected(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-sources-2", Email: "dup@example.com", Name: "Dup User",
		},
	}
	app, queries := setupE2EApp(t, oauth, http.DefaultClient)
	token := loginViaCallback(t, app, queries)

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

// ── Re-create after soft delete (status convention / uniqueness) ───────────────

func TestE2E_Sources_RecreateAfterSoftDelete(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-sources-6", Email: "recreate@example.com", Name: "Recreate User",
		},
	}
	app, queries := setupE2EApp(t, oauth, http.DefaultClient)
	token := loginViaCallback(t, app, queries)

	body := `{"url":"https://recreate.com","url_rss":"https://recreate.com/rss.xml"}`

	// Create
	c1, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	c1.Header.Set("Content-Type", "application/json")
	c1.Header.Set("Authorization", "Bearer "+token)
	r1, err := app.Test(c1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, r1.StatusCode)
	var first map[string]any
	require.NoError(t, readJSON(r1, &first))

	// Soft-delete
	del, _ := http.NewRequest(http.MethodDelete, "/v1/sources/"+first["id"].(string), nil)
	del.Header.Set("Authorization", "Bearer "+token)
	rd, err := app.Test(del)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rd.StatusCode)

	// Re-create with same url/url_rss — must succeed
	c2, _ := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(body))
	c2.Header.Set("Content-Type", "application/json")
	c2.Header.Set("Authorization", "Bearer "+token)
	r2, err := app.Test(c2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, r2.StatusCode)
}

// ── Auth guard ────────────────────────────────────────────────────────────────

func TestE2E_Sources_RequiresAuth(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupE2EApp(t, external.MockGoogleOAuth{}, http.DefaultClient)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/sources/create", `{"url":"https://x.com","url_rss":"https://x.com/rss"}`},
		{http.MethodGet, "/v1/sources", ""},
		{http.MethodGet, "/v1/sources/some-id", ""},
		{http.MethodPut, "/v1/sources/some-id", `{"url":"https://x.com","url_rss":"https://x.com/rss"}`},
		{http.MethodDelete, "/v1/sources/some-id", ""},
		{http.MethodGet, "/v1/sources/rss_discovery?url=https://x.com", ""},
	} {
		var bodyReader *strings.Reader
		if tc.body != "" {
			bodyReader = strings.NewReader(tc.body)
		} else {
			bodyReader = strings.NewReader("")
		}
		req, err := http.NewRequest(tc.method, tc.path, bodyReader)
		require.NoError(t, err)
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "expected 401 for %s %s", tc.method, tc.path)
	}
}

// ── RSS Discovery ─────────────────────────────────────────────────────────────

func TestE2E_RSSDiscovery_FindsAndReturnsFeeds(t *testing.T) {
	requireNotProduction(t)

	mockHTTP := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://site.com":         {StatusCode: http.StatusOK, Body: external.SampleHTMLWithRSSLink},
		"https://site.com/rss.xml": {StatusCode: http.StatusOK, Body: external.SampleRSSFeed},
	})
	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-sources-3", Email: "rss@example.com", Name: "RSS User",
		},
	}
	app, queries := setupE2EApp(t, oauth, mockHTTP)
	token := loginViaCallback(t, app, queries)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://site.com", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	feeds := result["feeds"].([]any)
	assert.NotEmpty(t, feeds)
	assert.Equal(t, "https://site.com/rss.xml", feeds[0])
}

func TestE2E_RSSDiscovery_ReturnsEmptyWhenNotFound(t *testing.T) {
	requireNotProduction(t)

	mockHTTP := external.NewMockRSSClient(map[string]external.MockRSSResponse{
		"https://nofeeds.com": {StatusCode: http.StatusOK, Body: external.SampleHTMLNoFeed},
	})
	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-sources-4", Email: "nofeeds@example.com", Name: "No Feeds User",
		},
	}
	app, queries := setupE2EApp(t, oauth, mockHTTP)
	token := loginViaCallback(t, app, queries)

	req, err := http.NewRequest(http.MethodGet, "/v1/sources/rss_discovery?url=https://nofeeds.com", nil)
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

// ── Input validation ──────────────────────────────────────────────────────────

func TestE2E_Sources_ValidationErrors(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-sources-5", Email: "val@example.com", Name: "Val User",
		},
	}
	app, queries := setupE2EApp(t, oauth, http.DefaultClient)
	token := loginViaCallback(t, app, queries)

	tt := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing url", `{"url_rss":"https://x.com/rss"}`, http.StatusBadRequest},
		{"missing url_rss", `{"url":"https://x.com"}`, http.StatusBadRequest},
		{"invalid url scheme", `{"url":"ftp://x.com","url_rss":"https://x.com/rss"}`, http.StatusBadRequest},
		{"empty body", `{}`, http.StatusBadRequest},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/v1/sources/create", strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			readJSON(resp, &map[string]any{})
		})
	}
}

// Ensure testOAuth2Config is used if needed by e2e setup
var _ = testOAuth2Config
