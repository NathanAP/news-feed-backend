package main

import (
	"errors"
	"fmt"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// runDevUser creates the development user (with its preferences from examples.json) if it does not
// exist yet. Idempotent: a second run skips. The user carries a fixed google_id
// (schemas.DevUserGoogleID) so the dev-login command/endpoint can find it unambiguously.
func runDevUser(sc *seedCtx) (*report, error) {
	rep := &report{}

	// Validate the example preference enums up front: the seed bypasses the endpoint (where enum
	// validation normally lives), so a typo in examples.json must fail loudly, not persist garbage.
	if err := validateDevPreferences(sc.ex.UserPreferences); err != nil {
		return rep, err
	}

	err := sc.runTx(sc.ctx, func(q db.Querier) error {
		if _, err := sc.userCtrl.FindUserByGoogleID(sc.ctx, q, schemas.DevUserGoogleID); err == nil {
			rep.skip("dev user " + sc.ex.User.Email + " (already exists)")
			return nil
		} else if !errors.Is(err, controllers.ErrUserNotFound) {
			return err
		}

		user, err := sc.userCtrl.CreateUser(sc.ctx, q, schemas.DevUserGoogleID, sc.ex.User.Email, sc.ex.User.Name, sc.ex.User.Picture)
		if err != nil {
			return err
		}

		// CreateUser already created default preferences; override them with the example values.
		var languageToTranslate *enums.Language
		if sc.ex.UserPreferences.LanguageToTranslate != nil {
			lang := enums.Language(*sc.ex.UserPreferences.LanguageToTranslate)
			languageToTranslate = &lang
		}
		if _, err := sc.prefCtrl.Update(sc.ctx, q, user.ID, controllers.UpdatePreferencesParams{
			LanguageToTranslate: languageToTranslate,
			AIPersonality:       enums.AIPersonality(sc.ex.UserPreferences.AIPersonality),
		}); err != nil {
			return err
		}

		rep.add("dev user " + sc.ex.User.Email)
		return nil
	})
	return rep, err
}

// validateDevPreferences ensures the example preference values are valid enum members.
func validateDevPreferences(p exampleUserPreferences) error {
	// language_to_translate is optional (null = translation off); validate only when present.
	if p.LanguageToTranslate != nil && !enums.Language(*p.LanguageToTranslate).IsValid() {
		return fmt.Errorf("invalid language_to_translate %q in examples.json (want pt|en|es|fr|de|it)", *p.LanguageToTranslate)
	}
	if !enums.AIPersonality(p.AIPersonality).IsValid() {
		return fmt.Errorf("invalid ai_personality %q in examples.json (want fun|informative|mixed)", p.AIPersonality)
	}
	return nil
}
