package articles

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func MarkAsRead(articleCtrl controllers.ArticleControllerInterface, afCtrl controllers.ArticleFeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)
		id := c.Params("id")

		var hasFeed bool
		err := runTx(c.Context(), func(q db.Querier) error {
			// Ensure the article exists and is active before touching its read state.
			if _, err := articleCtrl.FindByID(c.Context(), q, id); err != nil {
				return err
			}

			var err error
			hasFeed, err = afCtrl.MarkAsRead(c.Context(), q, id, claims.UserID)
			return err
		})
		if err != nil {
			if isNotFoundError(err) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "article not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to mark article as read"})
		}

		// 200 when the user has the article in at least one feed (marked or already read).
		// 204 when the article is not in any of the user's feeds — nothing happened.
		if hasFeed {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}
