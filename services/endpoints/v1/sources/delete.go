package sources

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func DeleteSource(ctrl controllers.SourceControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		err := runTx(c.Context(), func(q db.Querier) error {
			_, findErr := ctrl.FindByID(c.Context(), q, id)
			if findErr != nil {
				return findErr
			}
			return ctrl.SoftDelete(c.Context(), q, id)
		})
		if err != nil {
			if errors.Is(err, controllers.ErrSourceNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "source not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete source"})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
