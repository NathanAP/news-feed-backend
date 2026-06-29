package articles_feeds_test

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

func setupApp(t *testing.T) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	articleCtrl := controllers.NewArticleController()
	afCtrl := controllers.NewArticleFeedController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	a := app.Group("/v1/articles")
	a.Put("/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(articleCtrl, afCtrl, runTx))...)
	a.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, afCtrl, runTx))...)

	return app, queries
}

// seedAll seeds a user, source, article, feed and article_feed record, returning the token
// and all relevant IDs.
func seedAll(t *testing.T, queries db.Querier) (token, articleID, feedID, afID string) {
	t.Helper()

	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: user.ID, GoogleID: user.GoogleID, Email: user.Email, Name: user.Name, Picture: user.Picture,
	})
	require.NoError(t, err)

	rt := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID: rt.ID, UserID: rt.UserID, ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)

	token, err = jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)

	source := fixtures.NewTestSource()
	_, err = queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID: source.ID, Url: source.Url, UrlRss: source.UrlRss,
	})
	require.NoError(t, err)

	article := fixtures.NewTestArticle()
	_, err = queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID: article.ID, Title: article.Title, Content: article.Content,
		UrlOriginal: article.UrlOriginal, Keywords: article.Keywords, SourceID: source.ID,
	})
	require.NoError(t, err)
	articleID = article.ID

	feed := fixtures.NewTestFeed(user.ID)
	_, err = queries.CreateFeed(t.Context(), db.CreateFeedParams{
		ID: feed.ID, Name: feed.Name, Keywords: feed.Keywords, UserID: user.ID,
	})
	require.NoError(t, err)
	feedID = feed.ID

	af := fixtures.NewTestArticleFeed(articleID, feedID)
	_, err = queries.CreateArticleFeed(t.Context(), db.CreateArticleFeedParams{
		ID: af.ID, ArticleID: articleID, FeedID: feedID,
	})
	require.NoError(t, err)
	afID = af.ID

	return token, articleID, feedID, afID
}

// ── mark-as-read ──────────────────────────────────────────────────────────────

func TestIntegration_MarkAsRead_Returns200_WhenInFeed(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	token, articleID, _, _ := seedAll(t, queries)

	req, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+articleID+"/read", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestIntegration_MarkAsRead_Returns204_WhenNotInFeed(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	// Seed user and article but NO articles_feeds link.
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: user.ID, GoogleID: user.GoogleID, Email: user.Email, Name: user.Name, Picture: user.Picture,
	})
	require.NoError(t, err)
	rt := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID: rt.ID, UserID: rt.UserID, ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)

	source := fixtures.NewTestSource()
	_, err = queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID: source.ID, Url: source.Url, UrlRss: source.UrlRss,
	})
	require.NoError(t, err)
	article := fixtures.NewTestArticle()
	_, err = queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID: article.ID, Title: article.Title, Content: article.Content,
		UrlOriginal: article.UrlOriginal, Keywords: article.Keywords, SourceID: source.ID,
	})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+article.ID+"/read", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestIntegration_MarkAsRead_Idempotent(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	token, articleID, _, _ := seedAll(t, queries)

	// First call: marks as read.
	req1, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+articleID+"/read", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	// Second call: still 200, no side-effects.
	req2, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+articleID+"/read", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestIntegration_MarkAsRead_CrossUser_Returns204(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	_, articleID, _, _ := seedAll(t, queries)

	// Different user — no link between this user and the article.
	userB := fixtures.NewTestUser()
	userB.ID = "01900000-0000-7000-8000-0000000000b2"
	userB.GoogleID = "google-b2"
	userB.Email = "b2@example.com"
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: userB.ID, GoogleID: userB.GoogleID, Email: userB.Email, Name: userB.Name, Picture: userB.Picture,
	})
	require.NoError(t, err)
	rtB := fixtures.NewTestRefreshToken(userB.ID)
	rtB.ID = "019000ff-0000-7000-8000-0000000000b2"
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID: rtB.ID, UserID: rtB.UserID, ExpiresAt: rtB.ExpiresAt,
	})
	require.NoError(t, err)
	tokenB, err := jwtmock.GenerateTestAccessToken(userB, rtB.ID)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+articleID+"/read", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, err := app.Test(req)
	require.NoError(t, err)
	// User B has no feed containing this article → 204 (nothing to mark).
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// ── GET enrichment ────────────────────────────────────────────────────────────

func TestIntegration_GetArticle_IsReadNull_WhenNotInFeed(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	// Seed user and article but no articles_feeds.
	user := fixtures.NewTestUser()
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: user.ID, GoogleID: user.GoogleID, Email: user.Email, Name: user.Name, Picture: user.Picture,
	})
	require.NoError(t, err)
	rt := fixtures.NewTestRefreshToken(user.ID)
	_, err = queries.CreateRefreshToken(t.Context(), db.CreateRefreshTokenParams{
		ID: rt.ID, UserID: rt.UserID, ExpiresAt: rt.ExpiresAt,
	})
	require.NoError(t, err)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)

	source := fixtures.NewTestSource()
	_, err = queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID: source.ID, Url: source.Url, UrlRss: source.UrlRss,
	})
	require.NoError(t, err)
	article := fixtures.NewTestArticle()
	_, err = queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID: article.ID, Title: article.Title, Content: article.Content,
		UrlOriginal: article.UrlOriginal, Keywords: article.Keywords, SourceID: source.ID,
	})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+article.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Nil(t, result["is_read"])
}

func TestIntegration_GetArticle_IsReadFalse_WhenUnread(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	token, articleID, _, _ := seedAll(t, queries)

	req, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+articleID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, false, result["is_read"])
}

func TestIntegration_GetArticle_IsReadTrue_AfterMarkAsRead(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	token, articleID, _, _ := seedAll(t, queries)

	// Mark as read.
	markReq, _ := http.NewRequest(http.MethodPut, "/v1/articles/"+articleID+"/read", nil)
	markReq.Header.Set("Authorization", "Bearer "+token)
	markResp, err := app.Test(markReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, markResp.StatusCode)

	// Now the GET should show is_read = true.
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+articleID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(getResp, &result))
	assert.Equal(t, true, result["is_read"])
}

// ── Soft-delete invisibility ───────────────────────────────────────────────────

func TestIntegration_SoftDeletedFeed_MakesArticleFeedInvisible(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupApp(t)
	token, articleID, feedID, _ := seedAll(t, queries)

	// Before soft-delete: is_read = false (article is in the feed).
	getReq1, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+articleID, nil)
	getReq1.Header.Set("Authorization", "Bearer "+token)
	getResp1, err := app.Test(getReq1)
	require.NoError(t, err)
	var before map[string]any
	require.NoError(t, readJSON(getResp1, &before))
	assert.Equal(t, false, before["is_read"])

	// Soft-delete the feed directly (no endpoint for this yet).
	require.NoError(t, queries.SoftDeleteFeedByIDAndUser(t.Context(), db.SoftDeleteFeedByIDAndUserParams{
		ID:     feedID,
		UserID: fixtures.NewTestUser().ID,
	}))

	// After soft-delete: is_read = null (article no longer appears in any active feed).
	getReq2, _ := http.NewRequest(http.MethodGet, "/v1/articles/"+articleID, nil)
	getReq2.Header.Set("Authorization", "Bearer "+token)
	getResp2, err := app.Test(getReq2)
	require.NoError(t, err)
	var after map[string]any
	require.NoError(t, readJSON(getResp2, &after))
	assert.Nil(t, after["is_read"])
}

// ensure strings import is used
var _ = strings.NewReader
