package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type invalidateRequest struct {
	RefreshTokenID string `json:"refresh_token_id"`
}

func Invalidate(refreshTokenCtrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		var req invalidateRequest
		if err := c.BodyParser(&req); err != nil || req.RefreshTokenID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "refresh_token_id is required",
			})
		}

		err := runTx(c.Context(), func(q db.Querier) error {
			return refreshTokenCtrl.Revoke(c.Context(), q, req.RefreshTokenID)
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
