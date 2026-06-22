package middlewares

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
)

func NewAuthMiddleware(jwtSecret []byte, refreshTokenCtrl controllers.RefreshTokenControllerInterface) []fiber.Handler {
	return []fiber.Handler{
		parseJWT(jwtSecret),
		validateSession(refreshTokenCtrl),
	}
}

func GetClaims(c *fiber.Ctx) *schemas.Claims {
	return c.Locals("claims").(*schemas.Claims)
}

func parseJWT(secret []byte) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &schemas.Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secret, nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		c.Locals("claims", claims)
		return c.Next()
	}
}

func validateSession(ctrl controllers.RefreshTokenControllerInterface) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetClaims(c)
		if claims.RefreshTokenID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		if _, err := ctrl.FindByID(c.Context(), claims.RefreshTokenID); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "session expired, please log in again",
			})
		}

		return c.Next()
	}
}
