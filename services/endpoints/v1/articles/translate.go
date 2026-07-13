package articles

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/sanitize"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TranslateArticle translates an article on demand into a target language, for display only. It is
// user-triggered (the client decides when) and reads the user's personality preference from the
// JWT. Translation is a read-only capability with no preference gate: the language_to_translate
// preference is only a client hint (whether to show the option) and never affects API behavior. It
// validates the target language, loads the article, and refuses to translate when the article's
// original language is unknown or equal to the target (400). The AI output is re-sanitized with the
// treatment HTML whitelist as a defense. It persists nothing (read-only). The client caches the result.
func TranslateArticle(articleCtrl controllers.ArticleControllerInterface, translator ai.Translator, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		target := c.Params("language")
		if !enums.Language(target).IsValid() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported target language"})
		}

		var article db.Article
		err := runTx(c.Context(), func(q db.Querier) error {
			var e error
			article, e = articleCtrl.FindByID(c.Context(), q, id)
			return e
		})
		if err != nil {
			if errors.Is(err, controllers.ErrArticleNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "article not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load article"})
		}

		if !article.LanguageOriginal.Valid {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "article original language is unknown"})
		}
		if article.LanguageOriginal.String == target {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "target language matches the article's original language"})
		}

		translation, err := translator.Translate(
			c.Context(),
			enums.Language(target).DisplayName(),
			string(claims.AIPersonality),
			article.Title,
			article.Content,
		)
		if err != nil {
			logger.Log("translation failed: "+err.Error(), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to translate article"})
		}

		return c.JSON(schemas.ArticleTranslationResponse{
			Title:            translation.Title,
			Content:          sanitize.Sanitize(translation.Content), // enforce the HTML whitelist on model output
			Language:         target,
			LanguageOriginal: article.LanguageOriginal.String,
		})
	}
}
