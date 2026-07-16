package judgement_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/judgement"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// stubJudger records whether it was called and returns a fixed score/error.
type stubJudger struct {
	score  int
	err    error
	called int
}

func (s *stubJudger) Judge(_ context.Context, _ []string, _ string, _ []string) (int, error) {
	s.called++
	return s.score, s.err
}

func candidate(feedKeywordsJSON string, overlap int) controllers.FeedCandidate {
	return controllers.FeedCandidate{
		Feed:         db.Feed{ID: "feed-1", Keywords: json.RawMessage(feedKeywordsJSON)},
		OverlapCount: overlap,
	}
}

func eval(judger *stubJudger) *judgement.Evaluator {
	return judgement.NewEvaluator(judger, 70, 0.30, 2) // threshold 70, auto-associate 30%, min 2 matches
}

func TestEvaluate_AutoAssociatesStrongOverlap(t *testing.T) {
	j := &stubJudger{}
	// 3 of 5 keywords overlap → 60% >= 30% → auto-associated, no AI.
	res, err := eval(j).Evaluate(context.Background(), []controllers.FeedCandidate{candidate(`["a","b","c","d","e"]`, 3)}, "title", []string{"a", "b", "c"})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, judgement.DecisionAutoAssociated, res[0].Decision)
	assert.True(t, res[0].Passed)
	assert.Equal(t, 0, j.called, "auto-associate must not call the AI")
}

func TestEvaluate_DiscardsSingleOverlap(t *testing.T) {
	j := &stubJudger{}
	// 1 of 5 keywords → below min matches (2) → discarded, no AI.
	res, err := eval(j).Evaluate(context.Background(), []controllers.FeedCandidate{candidate(`["a","b","c","d","e"]`, 1)}, "title", []string{"a"})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, judgement.DecisionDiscarded, res[0].Decision)
	assert.False(t, res[0].Passed)
	assert.Equal(t, 0, j.called, "discard must not call the AI")
}

func TestEvaluate_JudgesBorderlineOverlap(t *testing.T) {
	j := &stubJudger{score: 90}
	// 2 of 7 keywords → ~28% < 30% and >= 2 matches → borderline → AI judges (90 >= 70 → passed).
	res, err := eval(j).Evaluate(context.Background(), []controllers.FeedCandidate{candidate(`["a","b","c","d","e","f","g"]`, 2)}, "title", []string{"a", "b"})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, judgement.DecisionJudged, res[0].Decision)
	assert.Equal(t, 90, res[0].Score)
	assert.True(t, res[0].Passed)
	assert.Equal(t, 1, j.called)
}

func TestEvaluate_JudgedBelowThresholdDoesNotPass(t *testing.T) {
	j := &stubJudger{score: 50}
	res, err := eval(j).Evaluate(context.Background(), []controllers.FeedCandidate{candidate(`["a","b","c","d","e","f","g"]`, 2)}, "title", []string{"a", "b"})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, judgement.DecisionJudged, res[0].Decision)
	assert.False(t, res[0].Passed, "50 < 70 must not pass")
}
