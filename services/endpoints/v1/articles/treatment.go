package articles

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/ai"
)

// TreatArticle is a dry-run of the treatment step: it takes raw article data (as produced by
// discovery), runs the two AI calls (clean content, then keywords over the cleaned content) and
// returns the result. It does NOT persist anything. Open for now (admin-future). Note: it calls
// the AI provider for real, so it consumes quota.
func TreatArticle(aiClient ai.Client) fiber.Handler {
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

		treated, err := aiClient.Treat(c.Context(), req.Article.Title, req.Article.Content)
		if err != nil {
			logger.Log(fmt.Sprintf("treatment failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to treat article"})
		}

		keywords, err := aiClient.Keywords(c.Context(), req.Article.Title, treated)
		if err != nil {
			logger.Log(fmt.Sprintf("keywords failed: %v", err), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to assign keywords"})
		}

		return c.JSON(schemas.ArticleTreatmentResponse{Content: treated, Keywords: keywords})
	}
}
