package articles

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func DeleteArticle(ctrl controllers.ArticleControllerInterface, outboundCtrl controllers.ArticleOutboundLinkControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		err := runTx(c.Context(), func(q db.Querier) error {
			article, findErr := ctrl.FindByID(c.Context(), q, id)
			if findErr != nil {
				return findErr
			}
			// Pre-remove cleanup, inside the same transaction as the soft delete: every outbound link
			// pointing at this article's internal page falls back to the source URL. Without it, older
			// articles keep anchors to a page that answers 404 once this one is gone (reads exclude
			// removed articles). Runs BEFORE the delete so a failure rolls the whole thing back.
			if retargetErr := outboundCtrl.Retarget(
				c.Context(), q, outboundlinks.InternalHref(id), article.UrlOriginal,
			); retargetErr != nil {
				return retargetErr
			}
			return ctrl.SoftDelete(c.Context(), q, id)
		})
		if err != nil {
			if errors.Is(err, controllers.ErrArticleNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "article not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete article"})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
