package users

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
)

func GetPreferences() fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		return c.JSON(schemas.UserPreferencesResponse{
			Theme:            claims.Theme,
			Language:         claims.Language,
			TranslateContent: claims.TranslateContent,
			AIPersonality:    claims.AIPersonality,
		})
	}
}
