package articles_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

// setupJudgementApp wires the judgement endpoint against a real DB (so the layer-1 candidate query
// runs for real) and a supplied judger mock (so layer 2 is deterministic). Threshold is 70.
func setupJudgementApp(t *testing.T, judger ai.Judger) (*fiber.App, db.Querier) {
	t.Helper()

	database := testutils.SetupTestDB(t)
	queries := db.New(database)
	runTx := controllers.NewTransactionRunner(database)

	feedCtrl := controllers.NewFeedController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), refreshTokenCtrl, runTx)

	judgers := map[string]ai.Judger{"local": judger, "groq": judger, "gemini": judger}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/v1/articles/judgement", append(authMiddleware, articleendpoints.JudgeArticle(feedCtrl, judgers, "local", 70, runTx))...)

	return app, queries
}

// seedFeed inserts an active feed with the given keywords for the seeded user.
func seedFeed(t *testing.T, queries db.Querier, userID, keywords string) string {
	t.Helper()
	feed := fixtures.NewTestFeed(userID)
	created, err := queries.CreateFeed(t.Context(), db.CreateFeedParams{
		ID: feed.ID, Name: feed.Name, Keywords: keywords, UserID: userID,
	})
	require.NoError(t, err)
	return created.ID
}

func postJudgement(t *testing.T, app *fiber.App, token, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/v1/articles/judgement", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

func TestIntegration_Judgement_MatchesCandidateAndScores(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupJudgementApp(t, &external.MockAIClient{}) // default score 90
	token := seedUser(t, queries)
	userID := userIDFromToken(t, queries)
	feedID := seedFeed(t, queries, userID, `["metallica","rock","metal","music","concert"]`)

	// Article shares "rock" with the feed → layer 1 matches; mock scores 90 (>= 70) → passed.
	body := `{"article":{"title":"Rock news","content":"body","keywords":["rock","guitar","tour"]}}`
	resp := postJudgement(t, app, token, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, float64(1), result["candidate_count"])
	judgements := result["judgements"].([]any)
	require.Len(t, judgements, 1)
	first := judgements[0].(map[string]any)
	assert.Equal(t, feedID, first["feed_id"])
	assert.Equal(t, true, first["passed"])
}

func TestIntegration_Judgement_NoKeywordOverlapYieldsNoCandidates(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupJudgementApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	userID := userIDFromToken(t, queries)
	seedFeed(t, queries, userID, `["anime","naruto","cosplay","manga","japan"]`)

	// No shared keyword → layer 1 returns nothing, AI never runs.
	body := `{"article":{"title":"Rock news","content":"body","keywords":["rock","guitar","tour"]}}`
	resp := postJudgement(t, app, token, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, float64(0), result["candidate_count"])
	assert.Empty(t, result["judgements"])
}

func TestIntegration_Judgement_ExcludesSoftDeletedFeed(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupJudgementApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	userID := userIDFromToken(t, queries)
	feedID := seedFeed(t, queries, userID, `["metallica","rock","metal","music","concert"]`)

	require.NoError(t, queries.SoftDeleteFeedByIDAndUser(t.Context(), db.SoftDeleteFeedByIDAndUserParams{ID: feedID, UserID: userID}))

	body := `{"article":{"title":"Rock news","content":"body","keywords":["rock","guitar","tour"]}}`
	resp := postJudgement(t, app, token, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, float64(0), result["candidate_count"], "a soft-deleted feed must not be a candidate")
}

func TestIntegration_Judgement_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _ := setupJudgementApp(t, &external.MockAIClient{})
	body := `{"article":{"title":"T","content":"c","keywords":["rock"]}}`
	resp := postJudgement(t, app, "", body)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// userIDFromToken returns the id of the single seeded user (seedUser creates exactly one).
func userIDFromToken(t *testing.T, queries db.Querier) string {
	t.Helper()
	user := fixtures.NewTestUser()
	found, err := queries.FindUserByID(t.Context(), user.ID)
	require.NoError(t, err)
	return found.ID
}
