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
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	"github.com/nathanap/news-feed-backend/services/sanitize"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TranslateArticle translates an article on demand into the reader's target language, for display
// only. It is user-triggered (the client decides when) and reads both the personality and the target
// language from the user's preferences in the JWT: the target is language_to_translate. A null target
// means the user has no translation configured (the client hides the option), and the API cannot
// translate — it returns 400. It loads the article and also refuses when the article's original
// language is unknown or equal to the target (400). The AI output is re-sanitized with the treatment
// HTML whitelist as a defense. It persists nothing (read-only). The client caches the result.
func TranslateArticle(articleCtrl controllers.ArticleControllerInterface, outboundCtrl controllers.ArticleOutboundLinkControllerInterface, translator ai.Translator, runTx controllers.TransactionRunner, clientURL string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		// The translation target is the user's preference, not a URL parameter. Null = no target
		// configured, so there is nothing to translate into.
		if claims.LanguageToTranslate == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no translation target configured in preferences"})
		}
		target := string(*claims.LanguageToTranslate)
		if !enums.Language(target).IsValid() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported target language"})
		}

		var article db.Article
		var links []db.ArticleOutboundLink
		err := runTx(c.Context(), func(q db.Querier) error {
			var e error
			article, e = articleCtrl.FindByID(c.Context(), q, id)
			if e != nil {
				return e
			}
			links, e = outboundCtrl.ListByArticleIDs(c.Context(), q, []string{id})
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

		// Swap outbound-link ids back to real URLs BEFORE the AI sees the body: the id form is not a
		// URL and the model output is re-sanitized below, which would strip a bare-id href.
		content := outboundlinks.Resolve(article.Content, controllers.OutboundLinksByID(links), clientURL)

		translation, err := translator.Translate(
			c.Context(),
			enums.Language(target).DisplayName(),
			string(claims.AIPersonality),
			article.Title,
			content,
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
