package articles

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/ai"
)

// TreatArticle is a dry-run of the treatment step: it takes raw article data (as produced by
// discovery), runs treatment (LLM) then keyword naming, and returns the result plus per-step
// timings. The keyword-naming backend is chosen by the configured default mode, or overridden per
// call via the body's `keywords_mode` (local | groq | gemini) so backends can be benchmarked from
// Bruno without restarting. It does NOT persist anything, but calls the AI providers for real
// (consumes quota). Open for now (admin-future).
func TreatArticle(treater ai.Treater, keyworders map[string]ai.Keyworder, defaultMode string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.TreatArticleRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if strings.TrimSpace(req.Article.Title) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.title is required"})
		}
		if strings.TrimSpace(req.Article.Content) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article.content is required"})
		}

		mode := defaultMode
		if req.KeywordsMode != "" {
			mode = req.KeywordsMode
		}
		keyworder, ok := keyworders[mode]
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("unknown keywords_mode %q (available: %s)", mode, availableModes(keyworders)),
			})
		}

		treatStart := time.Now()
		treated, err := treater.Treat(c.Context(), req.Article.Title, req.Article.Content)
		treatmentMs := time.Since(treatStart).Milliseconds()
		if err != nil {
			logger.Log(fmt.Sprintf("treatment failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to treat article"})
		}

		kwStart := time.Now()
		keywords, err := keyworder.Keywords(c.Context(), req.Article.Title, treated)
		keywordsMs := time.Since(kwStart).Milliseconds()
		if err != nil {
			logger.Log(fmt.Sprintf("keywords failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to assign keywords"})
		}

		return c.JSON(schemas.ArticleTreatmentResponse{
			Content:      treated,
			Keywords:     keywords,
			KeywordsMode: mode,
			TreatmentMs:  treatmentMs,
			KeywordsMs:   keywordsMs,
		})
	}
}

func availableModes(keyworders map[string]ai.Keyworder) string {
	modes := make([]string, 0, len(keyworders))
	for m := range keyworders {
		modes = append(modes, m)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}
