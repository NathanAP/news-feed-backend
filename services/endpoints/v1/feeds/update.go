package feeds

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func UpdateFeed(ctrl controllers.FeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		var req schemas.UpdateFeedRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		if msg := validateFeedInput(req.Name, req.Keywords); msg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}

		var feed db.Feed
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			feed, err = ctrl.Update(c.Context(), q, id, claims.UserID, req.Name, req.Keywords)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrFeedNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "feed not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update feed"})
		}

		response, err := toFeedResponse(feed)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build feed response"})
		}

		return c.JSON(response)
	}
}
