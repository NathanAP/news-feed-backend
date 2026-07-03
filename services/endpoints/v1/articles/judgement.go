package articles

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/judgement"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// JudgeArticle is a dry-run of the judgement step: it takes a treated article (title, content and
// keywords) and, exactly as the CRON would, runs layer 1 (candidate feeds by keyword overlap, a
// read-only DB query) then layer 2 (AI relevance score per candidate) and reports each score with
// whether it cleared the threshold. The judgement backend is chosen by the configured default mode,
// or overridden per call via the body's `judgement_mode` (local | groq | gemini). It stops at the
// penultimate step: it does NOT write any articles_feeds association. It does call the AI for real
// (consumes quota) and reads real feeds. Open for now (admin-future).
func JudgeArticle(feedCtrl controllers.FeedControllerInterface, judgers map[string]ai.Judger, defaultMode string, threshold int, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.JudgeArticleRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if strings.TrimSpace(req.Article.Title) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.title is required"})
		}
		if strings.TrimSpace(req.Article.Content) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.content is required"})
		}
		if len(req.Article.Keywords) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.keywords is required"})
		}

		mode := defaultMode
		if req.JudgementMode != "" {
			mode = req.JudgementMode
		}
		judger, ok := judgers[mode]
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("unknown judgement_mode %q (available: %s)", mode, availableModes(judgers)),
			})
		}

		// Layer 1: candidate feeds by keyword overlap. Read-only, but wrapped in a transaction like
		// every DB access; nothing is written, so it is still a dry-run.
		var candidates []db.Feed
		if err := runTx(c.Context(), func(q db.Querier) error {
			var e error
			candidates, e = feedCtrl.FindCandidatesByKeywords(c.Context(), q, req.Article.Keywords)
			return e
		}); err != nil {
			logger.Log(fmt.Sprintf("candidate lookup failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to find candidate feeds"})
		}

		// Layer 2: AI scoring against each candidate.
		evaluator := judgement.NewEvaluator(judger, threshold)
		start := time.Now()
		results, err := evaluator.Evaluate(c.Context(), candidates, req.Article.Title, req.Article.Keywords, req.Article.Content)
		judgementMs := time.Since(start).Milliseconds()
		if err != nil {
			logger.Log(fmt.Sprintf("judgement failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to judge article"})
		}

		judgements := make([]schemas.FeedJudgement, 0, len(results))
		for _, r := range results {
			judgements = append(judgements, schemas.FeedJudgement{
				FeedID:   r.Feed.ID,
				FeedName: r.Feed.Name,
				Score:    r.Score,
				Passed:   r.Passed,
			})
		}

		return c.JSON(schemas.ArticleJudgementResponse{
			JudgementMode:  mode,
			Threshold:      threshold,
			CandidateCount: len(candidates),
			Judgements:     judgements,
			JudgementMs:    judgementMs,
		})
	}
}
