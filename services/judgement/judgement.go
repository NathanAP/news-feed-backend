// Package judgment is the second (AI) layer of feed judgement. Given a set of candidate feeds
// (already narrowed by the cheap keyword-overlap query) and an article, it asks the AI to score how
// strongly the article belongs to each feed and compares the score against a threshold. It performs
// no I/O beyond the AI calls: fetching candidates and persisting the resulting associations is the
// caller's responsibility, so the same Evaluator serves both the discovery CRON and the dry-run
// endpoint.
package judgment

import (
	"context"
	"fmt"

	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// Result is the outcome of judging one candidate feed against the article.
type Result struct {
	Feed   db.Feed
	Score  int
	Passed bool
}

// Evaluator scores articles against candidate feeds using a Judger and a pass threshold.
type Evaluator struct {
	judger    ai.Judger
	threshold int
}

// NewEvaluator builds an Evaluator. threshold is the inclusive minimum score (0-100) for a feed to
// be considered a match.
func NewEvaluator(judger ai.Judger, threshold int) *Evaluator {
	return &Evaluator{judger: judger, threshold: threshold}
}

// Threshold returns the configured pass threshold.
func (e *Evaluator) Threshold() int { return e.threshold }

// Evaluate scores the article against every candidate feed and marks those meeting the threshold.
// It fails fast on the first AI error so the caller can decide how to react (the endpoint returns
// 500; the CRON logs and skips the article). Candidates are typically the output of
// FeedController.FindCandidatesByKeywords.
func (e *Evaluator) Evaluate(ctx context.Context, candidates []db.Feed, title string, articleKeywords []string, content string) ([]Result, error) {
	results := make([]Result, 0, len(candidates))
	for _, feed := range candidates {
		feedKeywords, err := controllers.DecodeKeywords(feed.Keywords)
		if err != nil {
			return nil, fmt.Errorf("failed to decode feed %s keywords: %w", feed.ID, err)
		}

		score, err := e.judger.Judge(ctx, feedKeywords, title, articleKeywords, content)
		if err != nil {
			return nil, fmt.Errorf("failed to judge feed %s: %w", feed.ID, err)
		}

		results = append(results, Result{Feed: feed, Score: score, Passed: score >= e.threshold})
	}
	return results, nil
}
