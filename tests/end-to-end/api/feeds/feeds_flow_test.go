package feeds_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
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
	feedCtrl := controllers.NewFeedController()
	afCtrl := controllers.NewArticleFeedController()
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	auth := app.Group("/v1/auth")
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, []byte(jwtmock.TestJWTSecret)))

	f := app.Group("/v1/feeds")
	f.Post("/create", append(authMiddleware, feedendpoints.CreateFeed(feedCtrl, runTx))...)
	f.Get("/check-for-new-articles", append(authMiddleware, feedendpoints.CheckForNewArticles(afCtrl, runTx))...)
	f.Get("/:id/articles", append(authMiddleware, feedendpoints.FeedArticles(feedCtrl, afCtrl, runTx))...)
	f.Get("/:id", append(authMiddleware, feedendpoints.GetFeed(feedCtrl, runTx))...)
	f.Get("", append(authMiddleware, feedendpoints.ListFeeds(feedCtrl, runTx))...)
	f.Put("/:id", append(authMiddleware, feedendpoints.UpdateFeed(feedCtrl, runTx))...)
	f.Delete("/:id", append(authMiddleware, feedendpoints.DeleteFeed(feedCtrl, runTx))...)

	return app, queries
}

func loginViaCallback(t *testing.T, app *fiber.App, queries db.Querier) string {
	t.Helper()

	token, refreshTokenID := testutils.CompleteOAuthLogin(t, app, []byte(jwtmock.TestJWTSecret))

	// Sanity-check that the login persisted the session and user.
	rt, err := queries.FindRefreshTokenByID(context.Background(), refreshTokenID)
	require.NoError(t, err)
	_, err = queries.FindUserByID(context.Background(), rt.UserID)
	require.NoError(t, err)

	return token
}

func TestE2E_Feeds_FullCRUDFlow(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-feeds-1", Email: "feeds@example.com", Name: "Feeds User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)

	// Create
	createBody := `{"name":"My E2E Feed","keywords":["metallica","rock","metal","music","concert"]}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	id := created["id"].(string)
	assert.Equal(t, "My E2E Feed", created["name"])
	assert.True(t, created["status"].(bool))

	// Get
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	// List
	listReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	listed := decodePage(t, listResp)
	assert.Len(t, listed, 1)

	// Update
	updateBody := `{"name":"My E2E Feed (renamed)","keywords":["a","b","c","d","e"]}`
	updateReq, _ := http.NewRequest(http.MethodPut, "/v1/feeds/"+id, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)
	var updated map[string]any
	require.NoError(t, readJSON(updateResp, &updated))
	assert.Equal(t, "My E2E Feed (renamed)", updated["name"])

	// Delete
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Gone
	afterReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+id, nil)
	afterReq.Header.Set("Authorization", "Bearer "+token)
	afterResp, err := app.Test(afterReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, afterResp.StatusCode)
}

func TestE2E_Feeds_FetchArticles(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-feeds-articles", Email: "feedarticles@example.com", Name: "Feed Articles User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)

	// Create a feed.
	createBody := `{"name":"News Feed","keywords":["metallica","rock","metal","music","concert"]}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	feedID := created["id"].(string)

	// Seed a source + article and link the article to the feed (the discovery pipeline's output).
	_, err = queries.CreateSource(context.Background(), db.CreateSourceParams{
		ID: "01900000-0000-7000-8000-0000000e0001", Name: "E2E Source", Url: "https://e2e-src.example.com", UrlRss: "https://e2e-src.example.com/rss",
	})
	require.NoError(t, err)
	_, err = queries.CreateArticle(context.Background(), db.CreateArticleParams{
		ID: "01900000-0000-7000-8000-0000000e0002", Title: "E2E Article", Content: "content",
		UrlOriginal: "https://e2e-article.example.com", Keywords: "[]", SourceID: "01900000-0000-7000-8000-0000000e0001",
	})
	require.NoError(t, err)
	_, err = queries.CreateArticleFeed(context.Background(), db.CreateArticleFeedParams{
		ID: "01900000-0000-7000-8000-0000000e0003", ArticleID: "01900000-0000-7000-8000-0000000e0002", FeedID: feedID,
	})
	require.NoError(t, err)

	// Fetch the feed's articles: the seeded article shows up, unread.
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+feedID+"/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	docs := decodePage(t, resp)
	require.Len(t, docs, 1)
	assert.Equal(t, "E2E Article", docs[0]["title"])
	assert.Equal(t, false, docs[0]["is_read"])

	// The unread filter includes it; the read filter excludes it.
	readReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+feedID+"/articles?is_read=true", nil)
	readReq.Header.Set("Authorization", "Bearer "+token)
	readResp, err := app.Test(readReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, readResp.StatusCode)
	assert.Empty(t, decodePage(t, readResp))
}

func TestE2E_Feeds_CheckForNewArticles(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-feeds-check", Email: "check@example.com", Name: "Check User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)

	// No feeds yet: the poll returns an empty object.
	emptyReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/check-for-new-articles", nil)
	emptyReq.Header.Set("Authorization", "Bearer "+token)
	emptyResp, err := app.Test(emptyReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, emptyResp.StatusCode)
	var emptyBody map[string]int64
	require.NoError(t, readJSON(emptyResp, &emptyBody))
	assert.Empty(t, emptyBody)

	// Create a feed and link an unread article to it (the discovery pipeline's output).
	createBody := `{"name":"Check Feed","keywords":["a","b","c","d","e"]}`
	createReq, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	var created map[string]any
	require.NoError(t, readJSON(createResp, &created))
	feedID := created["id"].(string)

	_, err = queries.CreateSource(context.Background(), db.CreateSourceParams{
		ID: "01900000-0000-7000-8000-0000000c0001", Name: "Check Source", Url: "https://check-src.example.com", UrlRss: "https://check-src.example.com/rss",
	})
	require.NoError(t, err)
	_, err = queries.CreateArticle(context.Background(), db.CreateArticleParams{
		ID: "01900000-0000-7000-8000-0000000c0002", Title: "Check Article", Content: "content",
		UrlOriginal: "https://check-article.example.com", Keywords: "[]", SourceID: "01900000-0000-7000-8000-0000000c0001",
	})
	require.NoError(t, err)
	_, err = queries.CreateArticleFeed(context.Background(), db.CreateArticleFeedParams{
		ID: "01900000-0000-7000-8000-0000000c0003", ArticleID: "01900000-0000-7000-8000-0000000c0002", FeedID: feedID,
	})
	require.NoError(t, err)

	// Now the feed reports exactly one unread article.
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/check-for-new-articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]int64
	require.NoError(t, readJSON(resp, &body))
	require.Len(t, body, 1)
	assert.Equal(t, int64(1), body[feedID])

	// Read the article; the feed then drops out of the poll entirely. The articles routes are not
	// mounted in this feeds-only harness, so mark it read directly against the querier — the user id
	// comes from the account the OAuth mock logged in.
	user, err := queries.FindUserByGoogleID(context.Background(), "e2e-feeds-check")
	require.NoError(t, err)
	require.NoError(t, queries.MarkArticleAsReadForUser(context.Background(), db.MarkArticleAsReadForUserParams{
		ArticleID: "01900000-0000-7000-8000-0000000c0002",
		UserID:    user.ID,
	}))

	afterReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/check-for-new-articles", nil)
	afterReq.Header.Set("Authorization", "Bearer "+token)
	afterResp, err := app.Test(afterReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, afterResp.StatusCode)
	var afterBody map[string]int64
	require.NoError(t, readJSON(afterResp, &afterBody))
	assert.Empty(t, afterBody)
}

func TestE2E_Feeds_LimitEnforced(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-feeds-2", Email: "limit@example.com", Name: "Limit User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)

	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"name":"Feed %d","keywords":["a","b","c","d","e"]}`, i)
		req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		readJSON(resp, &map[string]any{})
	}

	body := `{"name":"Sixth","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestE2E_Feeds_RequiresAuth(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/feeds/create", `{"name":"F","keywords":["a","b","c","d","e"]}`},
		{http.MethodGet, "/v1/feeds", ""},
		{http.MethodGet, "/v1/feeds/check-for-new-articles", ""},
		{http.MethodGet, "/v1/feeds/some-id", ""},
		{http.MethodGet, "/v1/feeds/some-id/articles", ""},
		{http.MethodPut, "/v1/feeds/some-id", `{"name":"F","keywords":["a","b","c","d","e"]}`},
		{http.MethodDelete, "/v1/feeds/some-id", ""},
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

func TestE2E_Feeds_ValidationErrors(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-feeds-3", Email: "val@example.com", Name: "Val User",
		},
	}
	app, queries := setupE2EApp(t, oauth)
	token := loginViaCallback(t, app, queries)

	tt := []struct {
		name string
		body string
	}{
		{"missing name", `{"keywords":["a","b","c","d","e"]}`},
		{"too few keywords", `{"name":"F","keywords":["a","b"]}`},
		{"empty keyword", `{"name":"F","keywords":["a","","c","d","e"]}`},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			readJSON(resp, &map[string]any{})
		})
	}
}
