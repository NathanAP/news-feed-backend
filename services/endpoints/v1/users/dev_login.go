package users

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// DevLogin logs in the development seed user without Google OAuth, issuing an access_token and a
// refresh_token exactly like the real callback would. It is a DEVELOPMENT-ONLY convenience and is
// registered in main.go only when ENVIRONMENT=development, so it never exists in staging/production.
// The dev user must have been seeded first (task sud / seed-user-dev).
func DevLogin(
	userCtrl controllers.UserControllerInterface,
	prefCtrl controllers.UserPreferencesControllerInterface,
	refreshCtrl controllers.RefreshTokenControllerInterface,
	authCtrl controllers.AuthControllerInterface,
	runTx controllers.TransactionRunner,
	accessTokenExpirySeconds int,
) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var accessToken, refreshTokenID string
		err := runTx(c.Context(), func(q db.Querier) error {
			user, err := userCtrl.FindUserByGoogleID(c.Context(), q, schemas.DevUserGoogleID)
			if err != nil {
				return err
			}

			prefs, err := prefCtrl.FindByUserID(c.Context(), q, user.ID)
			if err != nil {
				return err
			}

			rt, err := refreshCtrl.Create(c.Context(), q, user.ID)
			if err != nil {
				return err
			}

			token, err := authCtrl.GenerateAccessToken(user, rt.ID, prefs)
			if err != nil {
				return err
			}

			accessToken = token
			refreshTokenID = rt.ID
			return nil
		})
		if err != nil {
			if errors.Is(err, controllers.ErrUserNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dev user not seeded; run `task sud` (seed-user-dev)"})
			}
			logger.Log("dev-login failed: "+err.Error(), logger.ColorRed)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to log in dev user"})
		}

		return c.JSON(schemas.AuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshTokenID,
			ExpiresIn:    accessTokenExpirySeconds,
		})
	}
}
