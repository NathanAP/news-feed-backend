package articles_test

import (
	"encoding/json"
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
// runs for real) and a supplied judger mock (so layer 2 is deterministic). Threshold 70, auto-associate
// ratio 0.30, min matches 2 (the defaults).
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
	app.Post("/v1/articles/judgement", append(authMiddleware, articleendpoints.JudgeArticle(feedCtrl, judgers, "local", 70, 0.30, 2, -1, runTx))...)

	return app, queries
}

// seedFeed inserts an active feed with the given keywords for the seeded user.
func seedFeed(t *testing.T, queries db.Querier, userID, keywords string) string {
	t.Helper()
	feed := fixtures.NewTestFeed(userID)
	created, err := queries.CreateFeed(t.Context(), db.CreateFeedParams{
		ID: feed.ID, Name: feed.Name, Keywords: json.RawMessage(keywords), UserID: userID,
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

func TestIntegration_Judgement_AutoAssociatesStrongOverlap(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupJudgementApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	userID := userIDFromToken(t, queries)
	feedID := seedFeed(t, queries, userID, `["metallica","rock","metal","music","concert"]`)

	// Article covers 3 of the feed's 5 keywords (rock,metal,music) → 60% >= 30% → auto-associated, no AI.
	body := `{"article":{"title":"Rock news","keywords":["rock","metal","music","guitar"]}}`
	resp := postJudgement(t, app, token, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, float64(1), result["candidate_count"])
	first := result["judgements"].([]any)[0].(map[string]any)
	assert.Equal(t, feedID, first["feed_id"])
	assert.Equal(t, float64(3), first["overlap"], "3 keywords overlapped")
	assert.Equal(t, "auto_associated", first["decision"])
	assert.Equal(t, true, first["passed"])
}

func TestIntegration_Judgement_JudgesBorderlineOverlap(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupJudgementApp(t, &external.MockAIClient{}) // default score 90
	token := seedUser(t, queries)
	userID := userIDFromToken(t, queries)
	// 8-keyword feed; the article covers 2 (rock,metal) → 25% < 30% but >= 2 matches → borderline → AI.
	feedID := seedFeed(t, queries, userID, `["rock","metal","thrash","bay area","kirk","james","cliff","1983"]`)

	body := `{"article":{"title":"Rock news","keywords":["rock","metal","guitar"]}}`
	resp := postJudgement(t, app, token, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	first := result["judgements"].([]any)[0].(map[string]any)
	assert.Equal(t, feedID, first["feed_id"])
	assert.Equal(t, float64(2), first["overlap"])
	assert.Equal(t, "judged", first["decision"])
	assert.Equal(t, float64(90), first["score"])
	assert.Equal(t, true, first["passed"])
}

func TestIntegration_Judgement_DiscardsSingleKeywordOverlap(t *testing.T) {
	requireNotProduction(t)

	app, queries := setupJudgementApp(t, &external.MockAIClient{})
	token := seedUser(t, queries)
	userID := userIDFromToken(t, queries)
	seedFeed(t, queries, userID, `["metallica","rock","metal","music","concert"]`)

	// Article shares only "rock" → 1 match < min 2 → discarded (still a candidate, but not passed).
	body := `{"article":{"title":"Rock news","keywords":["rock","guitar","tour"]}}`
	resp := postJudgement(t, app, token, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, float64(1), result["candidate_count"])
	first := result["judgements"].([]any)[0].(map[string]any)
	assert.Equal(t, float64(1), first["overlap"])
	assert.Equal(t, "discarded", first["decision"])
	assert.Equal(t, false, first["passed"])
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
