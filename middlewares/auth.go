package middlewares

import (
	"context"
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
		claims, err := parseAccessToken(c.Get("Authorization"), secret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		c.Locals("claims", claims)
		return c.Next()
	}
}

// errUnauthenticated is what every failed token read collapses to. The caller never learns whether
// the header was missing, malformed, signed with the wrong key or simply expired: all of them mean
// the same thing to a client, and telling them apart only helps someone probing the endpoint.
var errUnauthenticated = errors.New("unauthenticated request")

// parseAccessToken validates a bearer token and returns its claims. It is split out of the middleware
// so the maintenance guard can identify an administrator on a request that has not gone through the
// auth middleware yet (that guard runs globally, before any per-route authentication) without
// carrying a second, drifting copy of the parsing rules.
func parseAccessToken(authHeader string, secret []byte) (*schemas.Claims, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errUnauthenticated
	}

	claims := &schemas.Claims{}
	token, err := jwt.ParseWithClaims(strings.TrimPrefix(authHeader, "Bearer "), claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errUnauthenticated
	}

	return claims, nil
}

// checkSession reports whether the session behind an access token is still live: logout, invalidate
// and expiry all work by killing the refresh token, so a token that parses cleanly is not enough.
// Shared with the maintenance guard, which must not accept a logged-out administrator's token just
// because it has not expired yet.
func checkSession(ctx context.Context, ctrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner, refreshTokenID string) error {
	if refreshTokenID == "" {
		return controllers.ErrRefreshTokenNotFound
	}

	return runTx(ctx, func(q db.Querier) error {
		_, err := ctrl.FindByID(ctx, q, refreshTokenID)
		return err
	})
}

func validateSession(ctrl controllers.RefreshTokenControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetClaims(c)

		if err := checkSession(c.Context(), ctrl, runTx, claims.RefreshTokenID); err != nil {
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
