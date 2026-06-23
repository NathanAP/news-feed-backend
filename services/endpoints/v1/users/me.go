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
			ID:        claims.UserID,
			Email:     claims.Email,
			Name:      claims.Name,
			Picture:   claims.Picture,
			CreatedAt: claims.CreatedAt,
		})
	}
}
