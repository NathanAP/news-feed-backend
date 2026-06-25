package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func InvalidateAll(refreshTokenCtrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		userID := c.Query("user_id")
		if userID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "user_id is required",
			})
		}

		err := runTx(c.Context(), func(q db.Querier) error {
			return refreshTokenCtrl.RevokeAll(c.Context(), q, userID)
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
