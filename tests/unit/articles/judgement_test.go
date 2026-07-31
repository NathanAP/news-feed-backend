package articles_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

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
)

// judgeCandidate builds a layer-1 candidate with a feed carrying the given keywords and the given
// keyword-overlap count — the two inputs the triage uses.
func judgeCandidate(id string, feedKeywords []string, overlap int) controllers.FeedCandidate {
	feed := fixtures.NewTestFeed("01900000-0000-7000-8000-000000000009")
	feed.ID = id
	kw, _ := json.Marshal(feedKeywords)
	feed.Keywords = kw
	return controllers.FeedCandidate{Feed: feed, OverlapCount: overlap}
}

// mockJudgeFeedCtrl implements FeedControllerInterface for the judgement endpoint tests. Only
// FindCandidatesByKeywords is exercised; the rest satisfy the interface. By default it returns one
// borderline candidate (7 keywords, 2 overlapping → 28% < 30% and >= 2 matches) so layer 2 (AI) runs.
type mockJudgeFeedCtrl struct {
	findCandidatesFn func(ctx context.Context, q db.Querier, keywords []string, inactiveDays int) ([]controllers.FeedCandidate, error)
}

func (m *mockJudgeFeedCtrl) Create(_ context.Context, _ db.Querier, userID, _ string, _ []string) (db.Feed, error) {
	return fixtures.NewTestFeed(userID), nil
}
func (m *mockJudgeFeedCtrl) FindByID(_ context.Context, _ db.Querier, _, userID string) (db.Feed, error) {
	return fixtures.NewTestFeed(userID), nil
}
func (m *mockJudgeFeedCtrl) FindCandidatesByKeywords(ctx context.Context, q db.Querier, keywords []string, inactiveDays int) ([]controllers.FeedCandidate, error) {
	if m.findCandidatesFn != nil {
		return m.findCandidatesFn(ctx, q, keywords, inactiveDays)
	}
	return []controllers.FeedCandidate{
		judgeCandidate("01900000-0000-7000-8000-000000000001", []string{"a", "b", "c", "d", "e", "f", "g"}, 2),
	}, nil
}
func (m *mockJudgeFeedCtrl) List(_ context.Context, _ db.Querier, userID string, _ controllers.ListFeedsFilter) ([]db.Feed, int64, error) {
	return []db.Feed{fixtures.NewTestFeed(userID)}, 1, nil
}
func (m *mockJudgeFeedCtrl) ListAll(_ context.Context, _ db.Querier, userID string) ([]db.Feed, error) {
	return []db.Feed{fixtures.NewTestFeed(userID)}, nil
}
func (m *mockJudgeFeedCtrl) Update(_ context.Context, _ db.Querier, _, userID, _ string, _ []string) (db.Feed, error) {
	return fixtures.NewTestFeed(userID), nil
}
func (m *mockJudgeFeedCtrl) SoftDelete(_ context.Context, _ db.Querier, _, _ string) error {
	return nil
}

var _ controllers.FeedControllerInterface = (*mockJudgeFeedCtrl)(nil)

func judgementApp(feedCtrl controllers.FeedControllerInterface, judger ai.Judger) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	authMiddleware := middlewares.NewAuthMiddleware([]byte(jwtmock.TestJWTSecret), &mockRefreshTokenCtrl{}, fakeTxRunner)
	judgers := map[string]ai.Judger{"local": judger, "groq": judger, "gemini": judger}
	app.Post("/v1/articles/judgement", append(authMiddleware, articleendpoints.JudgeArticle(feedCtrl, judgers, "local", 70, 0.30, 2, -1, fakeTxRunner))...)
	return app
}

func postJudgement(t *testing.T, app *fiber.App, body string, authed bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/v1/articles/judgement", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authed {
		req.Header.Set("Authorization", authHeader(t))
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

const validJudgeBody = `{"article":{"title":"Some News","keywords":["alpha","beta","gamma","delta","epsilon"]}}`

func TestJudgeArticle_Success(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	resp := postJudgement(t, app, validJudgeBody, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "local", result["judgement_mode"])
	assert.Equal(t, float64(70), result["threshold"])
	assert.Equal(t, float64(1), result["candidate_count"])
	judgements := result["judgements"].([]any)
	require.Len(t, judgements, 1)
	first := judgements[0].(map[string]any)
	assert.Equal(t, "judged", first["decision"]) // borderline overlap → AI ran
	assert.Equal(t, float64(90), first["score"]) // mock default score
	assert.Equal(t, true, first["passed"])       // 90 >= 70
	_, hasMs := result["judgement_ms"]
	assert.True(t, hasMs)
}

func TestJudgeArticle_AutoAssociatesStrongOverlap(t *testing.T) {
	requireNotProduction(t)

	// 5-keyword feed with 3 overlapping (60% >= 30%) → auto-associated, no AI. The judger would error
	// if called, proving no AI ran.
	strong := &mockJudgeFeedCtrl{
		findCandidatesFn: func(_ context.Context, _ db.Querier, _ []string, _ int) ([]controllers.FeedCandidate, error) {
			return []controllers.FeedCandidate{
				judgeCandidate("01900000-0000-7000-8000-00000000000a", []string{"a", "b", "c", "d", "e"}, 3),
			}, nil
		},
	}
	failIfCalled := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 0, ai.ErrInvalidScore
		},
	}
	app := judgementApp(strong, failIfCalled)
	resp := postJudgement(t, app, validJudgeBody, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode) // no 500 → AI was not called

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	first := result["judgements"].([]any)[0].(map[string]any)
	assert.Equal(t, "auto_associated", first["decision"])
	assert.Equal(t, true, first["passed"])
}

func TestJudgeArticle_DiscardsSingleKeywordOverlap(t *testing.T) {
	requireNotProduction(t)

	// 1 overlapping keyword (< min 2) → discarded, no AI.
	weak := &mockJudgeFeedCtrl{
		findCandidatesFn: func(_ context.Context, _ db.Querier, _ []string, _ int) ([]controllers.FeedCandidate, error) {
			return []controllers.FeedCandidate{
				judgeCandidate("01900000-0000-7000-8000-00000000000b", []string{"a", "b", "c", "d", "e"}, 1),
			}, nil
		},
	}
	failIfCalled := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 0, ai.ErrInvalidScore
		},
	}
	app := judgementApp(weak, failIfCalled)
	resp := postJudgement(t, app, validJudgeBody, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	first := result["judgements"].([]any)[0].(map[string]any)
	assert.Equal(t, "discarded", first["decision"])
	assert.Equal(t, false, first["passed"])
}

func TestJudgeArticle_ModeOverride(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	body := `{"judgement_mode":"gemini","article":{"title":"T","keywords":["a","b","c","d","e"]}}`
	resp := postJudgement(t, app, body, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, "gemini", result["judgement_mode"])
}

func TestJudgeArticle_BelowThreshold(t *testing.T) {
	requireNotProduction(t)

	lowScore := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 40, nil
		},
	}
	app := judgementApp(&mockJudgeFeedCtrl{}, lowScore)
	resp := postJudgement(t, app, validJudgeBody, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	first := result["judgements"].([]any)[0].(map[string]any)
	assert.Equal(t, false, first["passed"]) // 40 < 70
}

func TestJudgeArticle_NoCandidates(t *testing.T) {
	requireNotProduction(t)

	noCandidates := &mockJudgeFeedCtrl{
		findCandidatesFn: func(_ context.Context, _ db.Querier, _ []string, _ int) ([]controllers.FeedCandidate, error) {
			return []controllers.FeedCandidate{}, nil
		},
	}
	app := judgementApp(noCandidates, &external.MockAIClient{})
	resp := postJudgement(t, app, validJudgeBody, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, float64(0), result["candidate_count"])
	assert.Empty(t, result["judgements"])
}

func TestJudgeArticle_UnknownMode(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	body := `{"judgement_mode":"bogus","article":{"title":"T","keywords":["a","b","c","d","e"]}}`
	resp := postJudgement(t, app, body, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestJudgeArticle_MissingTitle(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	resp := postJudgement(t, app, `{"article":{"keywords":["a","b","c","d","e"]}}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestJudgeArticle_MissingKeywords(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	resp := postJudgement(t, app, `{"article":{"title":"T"}}`, true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestJudgeArticle_InvalidBody(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	resp := postJudgement(t, app, "not json", true)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestJudgeArticle_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := judgementApp(&mockJudgeFeedCtrl{}, &external.MockAIClient{})
	resp := postJudgement(t, app, validJudgeBody, false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJudgeArticle_AIFailure(t *testing.T) {
	requireNotProduction(t)

	failing := &external.MockAIClient{
		JudgeFn: func(_ context.Context, _ []string, _ string, _ []string) (int, error) {
			return 0, ai.ErrInvalidScore
		},
	}
	app := judgementApp(&mockJudgeFeedCtrl{}, failing)
	resp := postJudgement(t, app, validJudgeBody, true)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
