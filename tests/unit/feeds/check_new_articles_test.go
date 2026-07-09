package feeds_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func buildCheckNewArticlesApp(afCtrl controllers.ArticleFeedControllerInterface) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	f := app.Group("/v1/feeds")
	f.Get("/check-for-new-articles", append(authMiddleware, feedendpoints.CheckForNewArticles(afCtrl, fakeTxRunner))...)
	return app
}

func getCheckNewArticles(t *testing.T, app *fiber.App, withAuth bool) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/check-for-new-articles", nil)
	if withAuth {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestUnit_CheckForNewArticles_Unauthenticated_401(t *testing.T) {
	requireNotProduction(t)

	app := buildCheckNewArticlesApp(&mockAFCtrl{})
	resp := getCheckNewArticles(t, app, false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUnit_CheckForNewArticles_EmptyIsEmptyObject(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{countUnreadFn: func(_ context.Context, _ db.Querier, _ string) ([]db.CountUnreadArticlesByFeedForUserRow, error) {
		return []db.CountUnreadArticlesByFeedForUserRow{}, nil
	}}
	app := buildCheckNewArticlesApp(afCtrl)

	resp := getCheckNewArticles(t, app, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]int64
	require.NoError(t, readJSON(resp, &body))
	assert.Empty(t, body)
}

func TestUnit_CheckForNewArticles_ReturnsCountsKeyedByFeedID(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{countUnreadFn: func(_ context.Context, _ db.Querier, _ string) ([]db.CountUnreadArticlesByFeedForUserRow, error) {
		return []db.CountUnreadArticlesByFeedForUserRow{
			{FeedID: "feed-a", UnreadCount: 3},
			{FeedID: "feed-b", UnreadCount: 1},
		}, nil
	}}
	app := buildCheckNewArticlesApp(afCtrl)

	resp := getCheckNewArticles(t, app, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]int64
	require.NoError(t, readJSON(resp, &body))
	require.Len(t, body, 2)
	assert.Equal(t, int64(3), body["feed-a"])
	assert.Equal(t, int64(1), body["feed-b"])
}

func TestUnit_CheckForNewArticles_ControllerError_500(t *testing.T) {
	requireNotProduction(t)

	afCtrl := &mockAFCtrl{countUnreadFn: func(_ context.Context, _ db.Querier, _ string) ([]db.CountUnreadArticlesByFeedForUserRow, error) {
		return nil, assertErr{}
	}}
	app := buildCheckNewArticlesApp(afCtrl)

	resp := getCheckNewArticles(t, app, true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
