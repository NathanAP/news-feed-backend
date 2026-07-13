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
	"github.com/nathanap/news-feed-backend/services/langdetect"
	"github.com/nathanap/news-feed-backend/services/sanitize"
)

// TreatArticle is a dry-run of the treatment step: it takes raw article data (as produced by
// discovery), detects the language, sanitizes the raw body to the safe-HTML whitelist (deterministic,
// no AI) then names keywords, and returns the result plus per-step timings. The keyword-naming
// backend is chosen by the configured default mode, or overridden per call via the body's
// `keywords_mode` (local | groq | gemini) so backends can be benchmarked from Bruno without
// restarting. It does NOT persist anything, but calls the keyword AI for real (consumes quota). Open
// for now (admin-future).
func TreatArticle(keyworders map[string]ai.Keyworder, defaultMode string, detector langdetect.Detector) fiber.Handler {
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

		// Language detection mirrors the CRON step: lingua-go over the raw content, empty when
		// detection is not reliable. Not AI, so it does not consume quota.
		languageOriginal, _ := detector.Detect(req.Article.Title, req.Article.Content)

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

		// Body treatment is now deterministic: sanitize the raw RSS HTML to the safe whitelist. No AI,
		// no error path. (The URL-treatment step lands in a later version and will run here too.)
		treatStart := time.Now()
		treated := sanitize.Sanitize(req.Article.Content)
		treatmentMs := time.Since(treatStart).Milliseconds()

		kwStart := time.Now()
		keywords, err := keyworder.Keywords(c.Context(), req.Article.Title, treated)
		keywordsMs := time.Since(kwStart).Milliseconds()
		if err != nil {
			logger.Log(fmt.Sprintf("keywords failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to assign keywords"})
		}

		return c.JSON(schemas.ArticleTreatmentResponse{
			Content:          treated,
			Keywords:         keywords,
			KeywordsMode:     mode,
			LanguageOriginal: languageOriginal,
			TreatmentMs:      treatmentMs,
			KeywordsMs:       keywordsMs,
		})
	}
}

// availableModes lists the mode keys of an AI-backend map (keyworders, judgers), sorted, for error
// messages. Generic so it serves every per-mode map without duplication.
func availableModes[T any](backends map[string]T) string {
	modes := make([]string, 0, len(backends))
	for m := range backends {
		modes = append(modes, m)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}
