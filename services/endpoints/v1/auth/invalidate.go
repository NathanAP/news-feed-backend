package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

type invalidateRequest struct {
	RefreshTokenID string `json:"refresh_token_id"`
}

func Invalidate(refreshTokenCtrl controllers.RefreshTokenControllerInterface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req invalidateRequest
		if err := c.BodyParser(&req); err != nil || req.RefreshTokenID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "refresh_token_id is required",
			})
		}

		if err := refreshTokenCtrl.Revoke(c.Context(), req.RefreshTokenID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
