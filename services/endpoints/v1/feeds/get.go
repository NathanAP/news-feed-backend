package feeds

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func GetFeed(ctrl controllers.FeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		var feed db.Feed
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			feed, err = ctrl.FindByID(c.Context(), q, id, claims.UserID)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrFeedNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "feed not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve feed"})
		}

		response, err := toFeedResponse(feed)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build feed response"})
		}

		return c.JSON(response)
	}
}
