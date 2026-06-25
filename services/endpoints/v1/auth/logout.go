package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func Logout(refreshTokenCtrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		claims := middlewares.GetClaims(c)

		err := runTx(c.Context(), func(q db.Querier) error {
			return refreshTokenCtrl.Revoke(c.Context(), q, claims.RefreshTokenID)
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
