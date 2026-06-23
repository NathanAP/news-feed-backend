package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

func Logout(refreshTokenCtrl controllers.RefreshTokenControllerInterface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		claims := middlewares.GetClaims(c)

		if err := refreshTokenCtrl.Revoke(c.Context(), claims.RefreshTokenID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
