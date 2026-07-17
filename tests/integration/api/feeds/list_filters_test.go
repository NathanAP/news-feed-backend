package feeds_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// Filtering and pagination for GET /v1/feeds and GET /v1/feeds/{id}/articles happen in SQL, so they
// can only be proven against a real Postgres: these tests drive the endpoints over a live database
// and assert on the rows that actually come back. The unit tests only prove the handler forwards the
// right filter.

func listFeedsReq(t *testing.T, app *fiber.App, token, query string) ([]map[string]any, pagination.Meta) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/v1/feeds"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return decodeEnvelope(t, resp)
}

func decodeEnvelope(t *testing.T, resp *http.Response) ([]map[string]any, pagination.Meta) {
	t.Helper()
	var env struct {
		Docs       []map[string]any `json:"docs"`
		Pagination pagination.Meta  `json:"pagination"`
	}
	require.NoError(t, readJSON(resp, &env))
	return env.Docs, env.Pagination
}

// setArticleCreatedAt forces one article's created_at. The API has no way to set it (it is
// CURRENT_TIMESTAMP on insert), so the period-filter tests reach past the API on purpose.
func setArticleCreatedAt(t *testing.T, database *sql.DB, articleID string, at time.Time) {
	t.Helper()
	_, err := database.ExecContext(t.Context(), "UPDATE articles SET created_at = $1 WHERE id = $2", at, articleID)
	require.NoError(t, err)
}

// ── GET /v1/feeds ────────────────────────────────────────────────────────────

// The single-match case lives in feeds_test.go (TestIntegration_ListFeeds_FilterByName). This one
// covers the substring matching several rows, and asserts the total counts matches rather than rows.
func TestIntegration_ListFeeds_FilterByNameMatchesSubstringAcrossRows(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "60")

	createFeed(t, app, token, "Metal News")
	createFeed(t, app, token, "Anime News")
	createFeed(t, app, token, "Metal Reviews")

	docs, meta := listFeedsReq(t, app, token, "?name=Metal")
	require.Len(t, docs, 2)
	assert.Equal(t, int64(2), meta.TotalCount)
	for _, doc := range docs {
		assert.Contains(t, doc["name"], "Metal")
	}
}

func TestIntegration_ListFeeds_FilterByNameIsCaseInsensitive(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "61")

	createFeed(t, app, token, "Metal News")

	docs, _ := listFeedsReq(t, app, token, "?name=mEtAl")
	assert.Len(t, docs, 1)
}

func TestIntegration_ListFeeds_FilterMatchingNothingIsEmpty200(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "62")

	createFeed(t, app, token, "Metal News")

	docs, meta := listFeedsReq(t, app, token, "?name=nothing-matches")
	assert.Empty(t, docs)
	assert.NotNil(t, docs)
	assert.Equal(t, int64(0), meta.TotalCount)
}

// The filter must never widen the user scope: a feed is visible only to its owner, and that scope
// comes from the token. Another user's feed must not surface even when it matches the name.
func TestIntegration_ListFeeds_FilterNeverCrossesUsers(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	tokenA := seedUserVariant(t, queries, "63")
	tokenB := seedUserVariant(t, queries, "64")

	createFeed(t, app, tokenA, "Metal News")
	createFeed(t, app, tokenB, "Metal News")

	docs, meta := listFeedsReq(t, app, tokenA, "?name=Metal")
	assert.Len(t, docs, 1, "only the caller's own feed")
	assert.Equal(t, int64(1), meta.TotalCount, "the total must be scoped to the user too")
}

func TestIntegration_ListFeeds_Paginates(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "65")

	// A user may hold at most 5 active feeds (PROJECT.md), so 5 is the ceiling here.
	const total = 5
	for i := 0; i < total; i++ {
		createFeed(t, app, token, fmt.Sprintf("Feed %d", i))
	}

	docs, meta := listFeedsReq(t, app, token, "?page=1&page_size=2")
	assert.Len(t, docs, 2)
	assert.Equal(t, 3, meta.TotalPages)
	assert.Equal(t, int64(total), meta.TotalCount)
	assert.True(t, meta.HasNextPage)

	docs, meta = listFeedsReq(t, app, token, "?page=9&page_size=2")
	assert.Empty(t, docs)
	assert.Equal(t, 9, meta.ActualPage)
	assert.Equal(t, 3, meta.TotalPages)
}

// ── GET /v1/feeds/{id}/articles ──────────────────────────────────────────────

// The created_at window bounds the article, and both ends are inclusive: an article sitting exactly
// on a bound must be returned, not dropped.
func TestIntegration_FeedArticles_FilterByPeriod(t *testing.T) {
	requireNotProduction(t)

	app, queries, database := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "70")
	feedID := createFeed(t, app, token, "Dated Feed")
	sourceID := seedSource(t, queries, "70")

	june := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	august := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	juneID := seedArticleInFeed(t, queries, "p1", sourceID, feedID)
	julyID := seedArticleInFeed(t, queries, "p2", sourceID, feedID)
	augustID := seedArticleInFeed(t, queries, "p3", sourceID, feedID)
	setArticleCreatedAt(t, database, juneID, june)
	setArticleCreatedAt(t, database, julyID, july)
	setArticleCreatedAt(t, database, augustID, august)

	t.Run("starting_at keeps that date onwards", func(t *testing.T) {
		docs, meta := decodeEnvelope(t, getFeedArticles(t, app, token, feedID, "?period_starting_at=2026-06-15T00:00:00Z"))
		require.Len(t, docs, 2)
		assert.Equal(t, int64(2), meta.TotalCount)
	})

	t.Run("ending_at keeps up to that date", func(t *testing.T) {
		docs, _ := decodeEnvelope(t, getFeedArticles(t, app, token, feedID, "?period_ending_at=2026-06-15T00:00:00Z"))
		require.Len(t, docs, 1)
		assert.Equal(t, juneID, docs[0]["id"])
	})

	t.Run("both bounds form a window", func(t *testing.T) {
		docs, _ := decodeEnvelope(t, getFeedArticles(t, app, token, feedID,
			"?period_starting_at=2026-06-15T00:00:00Z&period_ending_at=2026-07-15T00:00:00Z"))
		require.Len(t, docs, 1)
		assert.Equal(t, julyID, docs[0]["id"])
	})

	t.Run("bounds are inclusive on both ends", func(t *testing.T) {
		// A window that starts and ends exactly on July's timestamp must still return July.
		docs, _ := decodeEnvelope(t, getFeedArticles(t, app, token, feedID,
			"?period_starting_at=2026-07-01T12:00:00Z&period_ending_at=2026-07-01T12:00:00Z"))
		require.Len(t, docs, 1)
		assert.Equal(t, julyID, docs[0]["id"])
	})

	t.Run("a window with no articles is an empty 200", func(t *testing.T) {
		docs, meta := decodeEnvelope(t, getFeedArticles(t, app, token, feedID,
			"?period_starting_at=2027-01-01T00:00:00Z"))
		assert.Empty(t, docs)
		assert.NotNil(t, docs)
		assert.Equal(t, int64(0), meta.TotalCount)
	})
}

// A bound sent in a non-UTC offset must be converted, not compared as if it were UTC: -03:00 puts
// this bound at 03:00Z, after the article, so the article must drop out.
func TestIntegration_FeedArticles_PeriodBoundIsConvertedToUTC(t *testing.T) {
	requireNotProduction(t)

	app, queries, database := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "71")
	feedID := createFeed(t, app, token, "TZ Feed")
	sourceID := seedSource(t, queries, "71")

	articleID := seedArticleInFeed(t, queries, "q1", sourceID, feedID)
	setArticleCreatedAt(t, database, articleID, time.Date(2026, 6, 15, 1, 0, 0, 0, time.UTC))

	// Same wall clock, different meaning: 00:00Z is before the article, 00:00-03:00 (= 03:00Z) is after.
	inUTC, _ := decodeEnvelope(t, getFeedArticles(t, app, token, feedID, "?period_starting_at=2026-06-15T00:00:00Z"))
	assert.Len(t, inUTC, 1)

	inOffset, _ := decodeEnvelope(t, getFeedArticles(t, app, token, feedID, "?period_starting_at=2026-06-15T00:00:00-03:00"))
	assert.Empty(t, inOffset, "the -03:00 bound is 03:00Z, which is after the article")
}

func TestIntegration_FeedArticles_Paginates(t *testing.T) {
	requireNotProduction(t)

	app, queries, _ := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "72")
	feedID := createFeed(t, app, token, "Paged Feed")
	sourceID := seedSource(t, queries, "72")

	const total = 5
	for i := 0; i < total; i++ {
		seedArticleInFeed(t, queries, fmt.Sprintf("r%d", i), sourceID, feedID)
	}

	docs, meta := decodeEnvelope(t, getFeedArticles(t, app, token, feedID, "?page=1&page_size=2"))
	assert.Len(t, docs, 2)
	assert.Equal(t, 3, meta.TotalPages)
	assert.Equal(t, int64(total), meta.TotalCount)
	assert.True(t, meta.HasNextPage)

	docs, meta = decodeEnvelope(t, getFeedArticles(t, app, token, feedID, "?page=9&page_size=2"))
	assert.Empty(t, docs)
	assert.Equal(t, 9, meta.ActualPage)
	assert.Equal(t, 3, meta.TotalPages)
	assert.Equal(t, int64(total), meta.TotalCount)
}

// is_read and the period window must compose, and the total must describe the combination.
func TestIntegration_FeedArticles_FiltersCombine(t *testing.T) {
	requireNotProduction(t)

	app, queries, database := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "73")
	feedID := createFeed(t, app, token, "Combo Feed")
	sourceID := seedSource(t, queries, "73")

	june := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	juneReadID := seedArticleInFeed(t, queries, "s1", sourceID, feedID)
	julyReadID := seedArticleInFeed(t, queries, "s2", sourceID, feedID)
	julyUnreadID := seedArticleInFeed(t, queries, "s3", sourceID, feedID)
	setArticleCreatedAt(t, database, juneReadID, june)
	setArticleCreatedAt(t, database, julyReadID, july)
	setArticleCreatedAt(t, database, julyUnreadID, july)

	for _, id := range []string{juneReadID, julyReadID} {
		require.NoError(t, queries.MarkArticleAsReadForUser(t.Context(), db.MarkArticleAsReadForUserParams{
			ArticleID: id,
			UserID:    userIDForSuffix("73"),
		}))
	}

	// Read AND from mid-June onwards: only July's read article qualifies.
	docs, meta := decodeEnvelope(t, getFeedArticles(t, app, token, feedID,
		"?is_read=true&period_starting_at=2026-06-15T00:00:00Z"))
	require.Len(t, docs, 1)
	assert.Equal(t, julyReadID, docs[0]["id"])
	assert.Equal(t, int64(1), meta.TotalCount)
}

// The same tie-break requirement as the global article list: articles of a feed arrive from the same
// CRON batch and share a created_at, so paging must still show each exactly once.
func TestIntegration_FeedArticles_PaginationIsStableAcrossTiedTimestamps(t *testing.T) {
	requireNotProduction(t)

	app, queries, database := setupIntegrationApp(t)
	token := seedUserVariant(t, queries, "74")
	feedID := createFeed(t, app, token, "Tied Feed")
	sourceID := seedSource(t, queries, "74")

	const total = 10
	tied := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	created := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		id := seedArticleInFeed(t, queries, fmt.Sprintf("t%d", i), sourceID, feedID)
		setArticleCreatedAt(t, database, id, tied)
		created[id] = struct{}{}
	}

	seen := make(map[string]int, total)
	for page := 1; page <= 5; page++ {
		docs, meta := decodeEnvelope(t, getFeedArticles(t, app, token, feedID, fmt.Sprintf("?page=%d&page_size=2", page)))
		require.Equal(t, int64(total), meta.TotalCount)
		for _, doc := range docs {
			seen[doc["id"].(string)]++
		}
	}

	require.Len(t, seen, total, "every article must appear exactly once across the pages")
	for id := range created {
		assert.Equal(t, 1, seen[id], "article %s appeared %d times across pages", id, seen[id])
	}
}
