package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func GoogleLogin(cfg *oauth2.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		state, err := uuid.NewV7()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}
		return c.Redirect(cfg.AuthCodeURL(state.String()), fiber.StatusTemporaryRedirect)
	}
}
