// Package judgement is the second layer of feed judgement. Given the candidate feeds from layer 1
// (keyword overlap) it triages each one cheaply before spending any AI: a strong overlap
// auto-associates, a trivial overlap is discarded, and only the borderline middle is scored by the
// AI (against the article title + keywords) versus a threshold. This keeps the AI cost bounded even
// when a popular article matches many feeds. It performs no I/O beyond the AI calls: fetching
// candidates and persisting associations is the caller's responsibility, so the same Evaluator
// serves both the discovery CRON and the dry-run endpoint.
package judgement

import (
	"context"
	"fmt"

	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// Decision is how a candidate feed was resolved by the triage.
type Decision string

const (
	// DecisionAutoAssociated: the article covers a strong fraction of the feed's keywords, so it is
	// associated without spending an AI call.
	DecisionAutoAssociated Decision = "auto_associated"
	// DecisionJudged: the overlap was borderline, so the AI scored it (Score/Passed are meaningful).
	DecisionJudged Decision = "judged"
	// DecisionDiscarded: the overlap was trivial (below the minimum), so it was dropped without AI.
	DecisionDiscarded Decision = "discarded"
)

// Result is the outcome of triaging (and possibly scoring) one candidate feed.
type Result struct {
	Feed         db.Feed
	OverlapCount int
	Decision     Decision
	Score        int  // meaningful only when Decision == DecisionJudged
	Passed       bool // true when the feed should be associated (auto-associated, or judged >= threshold)
}

// Evaluator triages candidate feeds and scores the borderline ones with a Judger.
//   - autoAssociateRatio: overlap/feedKeywords at or above this auto-associates (no AI).
//   - minMatches: a candidate with fewer overlapping keywords than this is discarded (no AI).
//   - threshold: the inclusive minimum AI score (0-100) for a judged feed to pass.
type Evaluator struct {
	judger             ai.Judger
	threshold          int
	autoAssociateRatio float64
	minMatches         int
}

// NewEvaluator builds an Evaluator with the triage/scoring configuration.
func NewEvaluator(judger ai.Judger, threshold int, autoAssociateRatio float64, minMatches int) *Evaluator {
	return &Evaluator{judger: judger, threshold: threshold, autoAssociateRatio: autoAssociateRatio, minMatches: minMatches}
}

// Threshold returns the configured pass threshold.
func (e *Evaluator) Threshold() int { return e.threshold }

// Evaluate triages every candidate and, for the borderline ones, asks the AI for a relevance score.
// It fails fast on the first AI error so the caller can decide how to react (the endpoint returns
// 500; the CRON logs and skips the article). Candidates are the output of
// FeedController.FindCandidatesByKeywords, each carrying its keyword-overlap count.
func (e *Evaluator) Evaluate(ctx context.Context, candidates []controllers.FeedCandidate, title string, articleKeywords []string) ([]Result, error) {
	results := make([]Result, 0, len(candidates))
	for _, cand := range candidates {
		feedKeywords, err := controllers.DecodeKeywords(cand.Feed.Keywords)
		if err != nil {
			return nil, fmt.Errorf("failed to decode feed %s keywords: %w", cand.Feed.ID, err)
		}

		res := Result{Feed: cand.Feed, OverlapCount: cand.OverlapCount}

		switch {
		case len(feedKeywords) > 0 && float64(cand.OverlapCount)/float64(len(feedKeywords)) >= e.autoAssociateRatio:
			res.Decision = DecisionAutoAssociated
			res.Passed = true
		case cand.OverlapCount < e.minMatches:
			res.Decision = DecisionDiscarded
		default:
			score, err := e.judger.Judge(ctx, feedKeywords, title, articleKeywords)
			if err != nil {
				return nil, fmt.Errorf("failed to judge feed %s: %w", cand.Feed.ID, err)
			}
			res.Decision = DecisionJudged
			res.Score = score
			res.Passed = score >= e.threshold
		}

		results = append(results, res)
	}
	return results, nil
}
