package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

func GoogleCallback(authCtrl controllers.AuthControllerInterface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		code := c.Query("code")
		if code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "authorization code is required",
			})
		}

		authResponse, err := authCtrl.HandleGoogleCallback(c.Context(), code)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "authentication failed",
			})
		}

		return c.JSON(authResponse)
	}
}
