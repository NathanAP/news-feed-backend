package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

func InvalidateAll(refreshTokenCtrl controllers.RefreshTokenControllerInterface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Query("user_id")
		if userID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "user_id is required",
			})
		}

		if err := refreshTokenCtrl.RevokeAll(c.Context(), userID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
