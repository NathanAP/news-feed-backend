package users

import (
	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type UpdatePreferencesResponse struct {
	AccessToken string                          `json:"access_token"`
	ExpiresIn   int                             `json:"expires_in"`
	Preferences schemas.UserPreferencesResponse `json:"preferences"`
}

func UpdatePreferences(prefCtrl controllers.UserPreferencesControllerInterface, authCtrl controllers.AuthControllerInterface, runTx controllers.TransactionRunner, accessTokenExpirySeconds int) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.UpdateUserPreferencesRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		// language_to_translate is optional (null clears translation); validate only when present.
		if req.LanguageToTranslate != nil && !req.LanguageToTranslate.IsValid() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid language_to_translate, accepted values: pt, en, es, fr, de, it",
			})
		}
		if !req.AIPersonality.IsValid() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid ai_personality, accepted values: fun, informative, mixed",
			})
		}

		claims := middlewares.GetClaims(c)

		var (
			updatedPrefs db.UserPreference
			newToken     string
		)
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			updatedPrefs, err = prefCtrl.Update(c.Context(), q, claims.UserID, controllers.UpdatePreferencesParams{
				LanguageToTranslate: req.LanguageToTranslate,
				AIPersonality:       req.AIPersonality,
			})
			if err != nil {
				return err
			}

			newToken, err = authCtrl.RegenerateFromClaims(c.Context(), q, claims, updatedPrefs)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.JSON(UpdatePreferencesResponse{
			AccessToken: newToken,
			ExpiresIn:   accessTokenExpirySeconds,
			Preferences: schemas.UserPreferencesResponse{
				LanguageToTranslate: controllers.LanguageToTranslatePtr(updatedPrefs),
				AIPersonality:       enums.AIPersonality(updatedPrefs.AiPersonality),
			},
		})
	}
}
