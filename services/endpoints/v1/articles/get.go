package articles

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func GetArticle(ctrl controllers.ArticleControllerInterface, afCtrl controllers.ArticleFeedControllerInterface, outboundCtrl controllers.ArticleOutboundLinkControllerInterface, runTx controllers.TransactionRunner, clientURL string) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		var article db.Article
		var afRecords []db.ArticlesFeed
		var links []db.ArticleOutboundLink
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			article, err = ctrl.FindByID(c.Context(), q, id)
			if err != nil {
				return err
			}

			// Enrich with the user's read state. An empty slice means the article is not
			// in any of the user's feeds — is_read will be nil in the response.
			afRecords, err = afCtrl.FindByArticleAndUser(c.Context(), q, id, claims.UserID)
			if err != nil {
				return err
			}

			// Outbound links for the read-time swap: the stored body carries their ids, not URLs.
			links, err = outboundCtrl.ListByArticleIDs(c.Context(), q, []string{id})
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrArticleNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "article not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve article"})
		}

		response, err := toArticleResponse(article)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build article response"})
		}
		response.Content = outboundlinks.Resolve(response.Content, controllers.OutboundLinksByID(links), clientURL)
		response.IsRead = controllers.IsReadState(afRecords)

		return c.JSON(response)
	}
}
