package feeds_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func getCheckNewArticles(t *testing.T, app *fiber.App, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/check-for-new-articles", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func decodeCounts(t *testing.T, resp *http.Response) map[string]int64 {
	t.Helper()
	var body map[string]int64
	require.NoError(t, readJSON(resp, &body))
	return body
}

func TestIntegration_CheckForNewArticles_CountsUnreadPerFeed(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "60")
	feedA := createFeed(t, app, token, "Feed A")
	feedB := createFeed(t, app, token, "Feed B")

	sourceID := seedSource(t, queries, "60")
	// Feed A: two unread articles. Feed B: one unread article.
	seedArticleInFeed(t, queries, "e1", sourceID, feedA)
	seedArticleInFeed(t, queries, "e2", sourceID, feedA)
	seedArticleInFeed(t, queries, "e3", sourceID, feedB)

	counts := decodeCounts(t, getCheckNewArticles(t, app, token))
	require.Len(t, counts, 2)
	assert.Equal(t, int64(2), counts[feedA])
	assert.Equal(t, int64(1), counts[feedB])
}

func TestIntegration_CheckForNewArticles_ReadArticlesAreNotCounted(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "61")
	feedID := createFeed(t, app, token, "Feed")

	sourceID := seedSource(t, queries, "61")
	readArticleID := seedArticleInFeed(t, queries, "f1", sourceID, feedID)
	seedArticleInFeed(t, queries, "f2", sourceID, feedID)

	// Two unread to start.
	require.Equal(t, int64(2), decodeCounts(t, getCheckNewArticles(t, app, token))[feedID])

	// Mark one read: count drops to 1.
	require.NoError(t, queries.MarkArticleAsReadForUser(t.Context(), db.MarkArticleAsReadForUserParams{
		ArticleID: readArticleID,
		UserID:    userIDForSuffix("61"),
	}))
	require.Equal(t, int64(1), decodeCounts(t, getCheckNewArticles(t, app, token))[feedID])
}

func TestIntegration_CheckForNewArticles_AllReadFeedIsOmitted(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "62")
	feedID := createFeed(t, app, token, "Feed")

	sourceID := seedSource(t, queries, "62")
	articleID := seedArticleInFeed(t, queries, "g1", sourceID, feedID)

	require.NoError(t, queries.MarkArticleAsReadForUser(t.Context(), db.MarkArticleAsReadForUserParams{
		ArticleID: articleID,
		UserID:    userIDForSuffix("62"),
	}))

	// The feed has no unread article, so it must not appear at all.
	counts := decodeCounts(t, getCheckNewArticles(t, app, token))
	_, present := counts[feedID]
	assert.False(t, present)
	assert.Empty(t, counts)
}

func TestIntegration_CheckForNewArticles_NoFeeds_EmptyObject(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "63")

	resp := getCheckNewArticles(t, app, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, decodeCounts(t, resp))
}

func TestIntegration_CheckForNewArticles_ScopedToUser(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	tokenA := seedUserVariant(t, queries, "64")
	tokenB := seedUserVariant(t, queries, "65")

	feedA := createFeed(t, app, tokenA, "A's Feed")
	sourceID := seedSource(t, queries, "64")
	seedArticleInFeed(t, queries, "h1", sourceID, feedA)

	// User A sees the unread; user B sees nothing (A's feed is not theirs).
	assert.Equal(t, int64(1), decodeCounts(t, getCheckNewArticles(t, app, tokenA))[feedA])
	assert.Empty(t, decodeCounts(t, getCheckNewArticles(t, app, tokenB)))
}

func TestIntegration_CheckForNewArticles_Unauthenticated_401(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t)
	resp := getCheckNewArticles(t, app, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
