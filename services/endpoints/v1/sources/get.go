package sources

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func GetSource(ctrl controllers.SourceControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		var source db.Source
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			source, err = ctrl.FindByID(c.Context(), q, id)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrSourceNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "source not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve source"})
		}

		return c.JSON(toSourceResponse(source))
	}
}
