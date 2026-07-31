package feeds_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

// The judgement candidate query (layer 1) skips feeds whose owner has gone inactive (0.43). These
// tests exercise that predicate directly against the real DB — two users own an identical feed, one
// active and one backdated past the window, and only the active one should surface.

func TestIntegration_CandidateFilter_SkipsInactiveUsers(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)
	feedCtrl := controllers.NewFeedController()

	const keywords = `["rock","metal","music","guitar","live"]`
	activeFeed := seedUserWithFeed(t, queries, "a1", keywords)
	inactiveFeed := seedUserWithFeed(t, queries, "b2", keywords)
	backdateLastActive(t, database, "01900000-0000-7000-8000-0000000000b2", 40) // 40 days > 15-day window

	articleKeywords := []string{"rock", "metal", "music"}

	// With a 15-day window, only the active user's feed is a candidate.
	withWindow := findCandidates(t, runTx, feedCtrl, articleKeywords, 15)
	assert.Contains(t, withWindow, activeFeed)
	assert.NotContains(t, withWindow, inactiveFeed, "inactive user's feed must be filtered out")

	// With the filter disabled (-1), inactivity is ignored and both feeds return.
	disabled := findCandidates(t, runTx, feedCtrl, articleKeywords, -1)
	assert.Contains(t, disabled, activeFeed)
	assert.Contains(t, disabled, inactiveFeed, "with the filter disabled, inactivity is ignored")
}

// A user active within the window still surfaces (the boundary that matters: recently-but-not-now).
func TestIntegration_CandidateFilter_KeepsRecentlyActiveUser(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)
	feedCtrl := controllers.NewFeedController()

	feedID := seedUserWithFeed(t, queries, "c3", `["rock","metal","music","guitar","live"]`)
	backdateLastActive(t, database, "01900000-0000-7000-8000-0000000000c3", 5) // 5 days < 15-day window

	candidates := findCandidates(t, runTx, feedCtrl, []string{"rock"}, 15)
	assert.Contains(t, candidates, feedID)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func findCandidates(t *testing.T, runTx controllers.TransactionRunner, feedCtrl *controllers.FeedController, keywords []string, inactiveDays int) []string {
	t.Helper()
	var candidates []controllers.FeedCandidate
	require.NoError(t, runTx(context.Background(), func(q db.Querier) error {
		var e error
		candidates, e = feedCtrl.FindCandidatesByKeywords(context.Background(), q, keywords, inactiveDays)
		return e
	}))
	ids := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.Feed.ID)
	}
	return ids
}

// seedUserWithFeed creates a user (active "now" by the DB default) and one feed with the given
// keywords, returning the feed id. The 2-hex suffix keeps the ids valid UUIDs and distinct per user.
func seedUserWithFeed(t *testing.T, queries db.Querier, suffix, keywords string) string {
	t.Helper()
	user := fixtures.NewTestUser()
	user.ID = "01900000-0000-7000-8000-0000000000" + suffix
	user.GoogleID = "google-" + suffix
	user.Email = suffix + "@example.com"
	_, err := queries.CreateUser(t.Context(), db.CreateUserParams{
		ID: user.ID, GoogleID: user.GoogleID, Email: user.Email, Name: user.Name, Picture: user.Picture,
	})
	require.NoError(t, err)

	feed := fixtures.NewTestFeed(user.ID)
	feed.ID = "019000fe-0000-7000-8000-0000000000" + suffix
	created, err := queries.CreateFeed(t.Context(), db.CreateFeedParams{
		ID: feed.ID, Name: feed.Name, Keywords: json.RawMessage(keywords), UserID: user.ID,
	})
	require.NoError(t, err)
	return created.ID
}

// backdateLastActive sets a user's last_active_at to `days` ago via raw SQL. There is no production
// query that sets an arbitrary activity time (the real one only stamps "now"), and simulating a stale
// user is exactly what these tests need.
func backdateLastActive(t *testing.T, database *sql.DB, userID string, days int) {
	t.Helper()
	_, err := database.ExecContext(context.Background(),
		"UPDATE users SET last_active_at = $1 WHERE id = $2",
		time.Now().UTC().AddDate(0, 0, -days), userID)
	require.NoError(t, err)
}
