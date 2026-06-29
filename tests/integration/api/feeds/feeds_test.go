package feeds_test

import (
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
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
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

	feedCtrl := controllers.NewFeedController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	f := app.Group("/v1/feeds")
	f.Post("/create", append(authMiddleware, feedendpoints.CreateFeed(feedCtrl, runTx))...)
	f.Get("/:id", append(authMiddleware, feedendpoints.GetFeed(feedCtrl, runTx))...)
	f.Get("", append(authMiddleware, feedendpoints.ListFeeds(feedCtrl, runTx))...)
	f.Put("/:id", append(authMiddleware, feedendpoints.UpdateFeed(feedCtrl, runTx))...)
	f.Delete("/:id", append(authMiddleware, feedendpoints.DeleteFeed(feedCtrl, runTx))...)

	return app, queries
}

// seedUserVariant seeds a user (and its refresh token) with a unique suffix and returns its token.
func seedUserVariant(t *testing.T, queries db.Querier, suffix string) string {
	t.Helper()
	user := fixtures.NewTestUser()
	user.ID = "01900000-0000-7000-8000-0000000000" + suffix
	user.GoogleID = "google-" + suffix
	user.Email = suffix + "@example.com"

	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	require.NoError(t, err)

	rt := fixtures.NewTestRefreshToken(user.ID)
	rt.ID = "019000ff-0000-7000-8000-0000000000" + suffix
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

func createFeed(t *testing.T, app *fiber.App, token, name string) string {
	t.Helper()
	body := `{"name":"` + name + `","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
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

func TestIntegration_CreateFeed_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "01")

	body := `{"name":"My Feed","keywords":["metallica","rock","metal","music","concert"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["id"])
	assert.Equal(t, "My Feed", result["name"])
	keywords := result["keywords"].([]any)
	assert.Len(t, keywords, 5)
}

func TestIntegration_CreateFeed_LimitReached(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "02")

	// Create 5 active feeds (the maximum).
	for i := 0; i < 5; i++ {
		createFeed(t, app, token, fmt.Sprintf("Feed %d", i))
	}

	// The 6th must be rejected with 409.
	body := `{"name":"Sixth","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestIntegration_CreateFeed_LimitFreedAfterDelete(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "03")

	var firstID string
	for i := 0; i < 5; i++ {
		id := createFeed(t, app, token, fmt.Sprintf("Feed %d", i))
		if i == 0 {
			firstID = id
		}
	}

	// Delete one → frees a slot.
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/"+firstID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Now a new feed is allowed again.
	body := `{"name":"Replacement","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// ── Cross-user isolation ────────────────────────────────────────────────────

func TestIntegration_Feeds_CrossUserIsolation(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	tokenA := seedUserVariant(t, queries, "0a")
	tokenB := seedUserVariant(t, queries, "0b")

	// User A creates a feed.
	feedID := createFeed(t, app, tokenA, "A's Feed")

	// User B must not be able to GET it.
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+feedID, nil)
	getReq.Header.Set("Authorization", "Bearer "+tokenB)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)

	// User B must not be able to UPDATE it.
	updBody := `{"name":"hijack","keywords":["a","b","c","d","e"]}`
	updReq, _ := http.NewRequest(http.MethodPut, "/v1/feeds/"+feedID, strings.NewReader(updBody))
	updReq.Header.Set("Content-Type", "application/json")
	updReq.Header.Set("Authorization", "Bearer "+tokenB)
	updResp, err := app.Test(updReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, updResp.StatusCode)

	// User B must not be able to DELETE it.
	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/"+feedID, nil)
	delReq.Header.Set("Authorization", "Bearer "+tokenB)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, delResp.StatusCode)

	// User B's list must not contain A's feed.
	listReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	listReq.Header.Set("Authorization", "Bearer "+tokenB)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	var listed []map[string]any
	require.NoError(t, readJSON(listResp, &listed))
	assert.Empty(t, listed)

	// User A still sees their own feed.
	ownReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+feedID, nil)
	ownReq.Header.Set("Authorization", "Bearer "+tokenA)
	ownResp, err := app.Test(ownReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, ownResp.StatusCode)
}

func TestIntegration_Feeds_LimitIsPerUser(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	tokenA := seedUserVariant(t, queries, "0c")
	tokenB := seedUserVariant(t, queries, "0d")

	// User A fills their 5 slots.
	for i := 0; i < 5; i++ {
		createFeed(t, app, tokenA, fmt.Sprintf("A Feed %d", i))
	}

	// User B is unaffected and can still create.
	body := `{"name":"B Feed","keywords":["a","b","c","d","e"]}`
	req, _ := http.NewRequest(http.MethodPost, "/v1/feeds/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// ── Update / Delete / List ──────────────────────────────────────────────────

func TestIntegration_UpdateFeed_Success(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "0e")
	id := createFeed(t, app, token, "Original")

	body := `{"name":"Updated","keywords":["x","y","z","w","v"]}`
	req, _ := http.NewRequest(http.MethodPut, "/v1/feeds/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "Updated", result["name"])
	assert.NotNil(t, result["modified_at"])
}

func TestIntegration_DeleteFeed_RemovesFromList(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "0f")
	id := createFeed(t, app, token, "Doomed")

	delReq, _ := http.NewRequest(http.MethodDelete, "/v1/feeds/"+id, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := app.Test(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, delResp.StatusCode)

	// Gone from GET and from list.
	getReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+id, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)

	listReq, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	var listed []map[string]any
	require.NoError(t, readJSON(listResp, &listed))
	assert.Empty(t, listed)
}

func TestIntegration_ListFeeds_FilterByName(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "1a")
	createFeed(t, app, token, "Metallica News")
	createFeed(t, app, token, "Anime News")

	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds?name=metallica", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, readJSON(resp, &result))
	require.Len(t, result, 1)
	assert.Equal(t, "Metallica News", result[0]["name"])
}

func TestIntegration_Feeds_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupIntegrationApp(t)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
