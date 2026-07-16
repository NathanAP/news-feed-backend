package middlewares

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewAuthMiddleware(jwtSecret []byte, refreshTokenCtrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner) []fiber.Handler {
	return []fiber.Handler{
		parseJWT(jwtSecret),
		validateSession(refreshTokenCtrl, runTx),
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

func validateSession(ctrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetClaims(c)
		if claims.RefreshTokenID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		err := runTx(c.Context(), func(q db.Querier) error {
			_, err := ctrl.FindByID(c.Context(), q, claims.RefreshTokenID)
			return err
		})
		if err != nil {
			// Only a genuinely absent/expired session is the user's problem (401). Anything else is
			// an infrastructure failure and must surface as 500: telling a client with a perfectly
			// valid session to "log in again" because the database blinked would sign every user out
			// at once, and the login they attempt next would fail too. conventions.md: exceptions
			// default to 500, and the typed errors exist precisely so this can be told apart.
			if errors.Is(err, controllers.ErrRefreshTokenNotFound) || errors.Is(err, controllers.ErrRefreshTokenExpired) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "session expired, please log in again",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.Next()
	}
}
