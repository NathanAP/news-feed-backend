package auth

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

func RefreshToken(authCtrl controllers.AuthControllerInterface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		var req schemas.RefreshRequest
		if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "refresh_token is required",
			})
		}

		authResponse, err := authCtrl.RefreshAccessToken(c.Context(), req.RefreshToken)
		if err != nil {
			if errors.Is(err, controllers.ErrRefreshTokenNotFound) || errors.Is(err, controllers.ErrRefreshTokenExpired) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "session expired, please log in again",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.JSON(authResponse)
	}
}
