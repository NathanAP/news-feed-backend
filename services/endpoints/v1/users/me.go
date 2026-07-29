package users

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
)

func GetMe() fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		claims := middlewares.GetClaims(c)

		return c.JSON(schemas.UserResponse{
			ID:      claims.UserID,
			Email:   claims.Email,
			Name:    claims.Name,
			Picture: claims.Picture,
			// Echoed from the token, like every other field here: this route is a decode of the
			// caller's own claims, not a database read. So a user promoted mid-session still reads
			// admin:false until the token is refreshed, while the API already honours the promotion.
			// The client only uses it to decide what to render, so the lag is cosmetic.
			Admin:     claims.Admin,
			CreatedAt: claims.CreatedAt.Time,
		})
	}
}
