package feeds_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// userIDForSuffix mirrors the deterministic user ID built by seedUserVariant, so a test can address
// the same user directly against the querier (e.g. to mark an article as read).
func userIDForSuffix(suffix string) string {
	return "01900000-0000-7000-8000-0000000000" + suffix
}

func seedSource(t *testing.T, queries db.Querier, suffix string) string {
	t.Helper()
	id := "01900000-0000-7000-8000-0000000030" + suffix
	_, err := queries.CreateSource(t.Context(), db.CreateSourceParams{
		ID:     id,
		Name:   "Source " + suffix,
		Url:    "https://src-" + suffix + ".example.com",
		UrlRss: "https://src-" + suffix + ".example.com/rss",
	})
	require.NoError(t, err)
	return id
}

// seedArticleInFeed creates an active article, links it to the given feed, and returns the article ID.
func seedArticleInFeed(t *testing.T, queries db.Querier, suffix, sourceID, feedID string) string {
	t.Helper()
	articleID := "01900000-0000-7000-8000-0000000040" + suffix
	_, err := queries.CreateArticle(t.Context(), db.CreateArticleParams{
		ID:          articleID,
		Title:       "Article " + suffix,
		Content:     "content " + suffix,
		UrlOriginal: "https://article-" + suffix + ".example.com",
		Keywords:    json.RawMessage("[]"),
		SourceID:    sourceID,
	})
	require.NoError(t, err)

	afID := "01900000-0000-7000-8000-0000000050" + suffix
	_, err = queries.CreateArticleFeed(t.Context(), db.CreateArticleFeedParams{
		ID:        afID,
		ArticleID: articleID,
		FeedID:    feedID,
	})
	require.NoError(t, err)
	return articleID
}

func getFeedArticles(t *testing.T, app *fiber.App, token, feedID, query string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/"+feedID+"/articles"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestIntegration_FeedArticles_ReturnsWithIsReadAndFilters(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "20")
	feedID := createFeed(t, app, token, "My Feed")

	sourceID := seedSource(t, queries, "20")
	readArticleID := seedArticleInFeed(t, queries, "a1", sourceID, feedID)
	seedArticleInFeed(t, queries, "a2", sourceID, feedID)

	// Both start unread.
	docs := decodePage(t, getFeedArticles(t, app, token, feedID, ""))
	require.Len(t, docs, 2)
	for _, d := range docs {
		assert.Equal(t, false, d["is_read"])
	}

	// Mark one as read for this user, then filter both ways.
	require.NoError(t, queries.MarkArticleAsReadForUser(t.Context(), db.MarkArticleAsReadForUserParams{
		ArticleID: readArticleID,
		UserID:    userIDForSuffix("20"),
	}))

	readDocs := decodePage(t, getFeedArticles(t, app, token, feedID, "?is_read=true"))
	require.Len(t, readDocs, 1)
	assert.Equal(t, readArticleID, readDocs[0]["id"])
	assert.Equal(t, true, readDocs[0]["is_read"])

	unreadDocs := decodePage(t, getFeedArticles(t, app, token, feedID, "?is_read=false"))
	require.Len(t, unreadDocs, 1)
	assert.Equal(t, false, unreadDocs[0]["is_read"])
}

func TestIntegration_FeedArticles_WithSourcesTrue_PopulatesSource(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "25")
	feedID := createFeed(t, app, token, "Sourced Feed")

	sourceID := seedSource(t, queries, "25")
	seedArticleInFeed(t, queries, "c1", sourceID, feedID)

	docs := decodePage(t, getFeedArticles(t, app, token, feedID, "?with_sources=true"))
	require.Len(t, docs, 1)
	source, ok := docs[0]["source"].(map[string]any)
	require.True(t, ok, "expected source to be populated")
	assert.Equal(t, sourceID, source["id"])
	assert.Equal(t, "Source 25", source["name"])
	assert.Equal(t, "https://src-25.example.com", source["url"])
	assert.Equal(t, "https://src-25.example.com/rss", source["url_rss"])
	assert.Equal(t, true, source["status"])
}

// Per conventions.md, with_{related_table_name} only populates on the literal value "true"; any
// other value (including absent) is silently ignored — never an error.
func TestIntegration_FeedArticles_WithSourcesNotTrue_OmitsSource(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "26")
	feedID := createFeed(t, app, token, "Unsourced Feed")

	sourceID := seedSource(t, queries, "26")
	seedArticleInFeed(t, queries, "d1", sourceID, feedID)

	absentResp := getFeedArticles(t, app, token, feedID, "")
	require.Equal(t, http.StatusOK, absentResp.StatusCode)
	absentDocs := decodePage(t, absentResp)
	require.Len(t, absentDocs, 1)
	assert.Nil(t, absentDocs[0]["source"])

	falseResp := getFeedArticles(t, app, token, feedID, "?with_sources=false")
	require.Equal(t, http.StatusOK, falseResp.StatusCode)
	falseDocs := decodePage(t, falseResp)
	require.Len(t, falseDocs, 1)
	assert.Nil(t, falseDocs[0]["source"])

	garbageResp := getFeedArticles(t, app, token, feedID, "?with_sources=yes")
	require.Equal(t, http.StatusOK, garbageResp.StatusCode)
	garbageDocs := decodePage(t, garbageResp)
	require.Len(t, garbageDocs, 1)
	assert.Nil(t, garbageDocs[0]["source"])
}

func TestIntegration_FeedArticles_OtherUsersFeed_404(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	tokenA := seedUserVariant(t, queries, "21")
	tokenB := seedUserVariant(t, queries, "22")

	feedID := createFeed(t, app, tokenA, "A's Feed")
	sourceID := seedSource(t, queries, "21")
	seedArticleInFeed(t, queries, "b1", sourceID, feedID)

	// User B must not read A's feed articles — 404, not leaking existence.
	resp := getFeedArticles(t, app, tokenB, feedID, "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestIntegration_FeedArticles_OwnedButEmpty_200(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "23")
	feedID := createFeed(t, app, token, "Empty Feed")

	resp := getFeedArticles(t, app, token, feedID, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	docs := decodePage(t, resp)
	assert.Empty(t, docs)
}

func TestIntegration_FeedArticles_FeedNotFound_404(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "24")

	resp := getFeedArticles(t, app, token, "01900000-0000-7000-8000-000000009999", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestIntegration_FeedArticles_Unauthenticated_401(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t)
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds/any/articles", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
