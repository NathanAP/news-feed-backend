package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var ErrUserPreferencesNotFound = errors.New("user preferences not found")

type UpdatePreferencesParams struct {
	Theme            enums.Theme
	Language         enums.Language
	TranslateContent bool
	AIPersonality    enums.AIPersonality
}

type UserPreferencesController struct{}

func NewUserPreferencesController() *UserPreferencesController {
	return &UserPreferencesController{}
}

// DefaultPreferencesParams builds the parameters for a user's default preferences,
// generating a fresh UUID v7. The default values live here so that any flow creating
// preferences uses a single source of truth.
func DefaultPreferencesParams(userID string) (db.CreateUserPreferencesParams, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.CreateUserPreferencesParams{}, fmt.Errorf("failed to generate preferences ID: %w", err)
	}

	return db.CreateUserPreferencesParams{
		ID:               id.String(),
		UserID:           userID,
		Theme:            string(enums.ThemeDark),
		Language:         string(enums.LanguagePT),
		TranslateContent: 1,
		AiPersonality:    string(enums.AIPersonalityMixed),
	}, nil
}

func (c *UserPreferencesController) CreateDefault(ctx context.Context, q db.Querier, userID string) (db.UserPreference, error) {
	params, err := DefaultPreferencesParams(userID)
	if err != nil {
		return db.UserPreference{}, err
	}

	prefs, err := q.CreateUserPreferences(ctx, params)
	if err != nil {
		return db.UserPreference{}, fmt.Errorf("failed to create user preferences: %w", err)
	}

	return prefs, nil
}

func (c *UserPreferencesController) FindByUserID(ctx context.Context, q db.Querier, userID string) (db.UserPreference, error) {
	prefs, err := q.FindUserPreferencesByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.UserPreference{}, ErrUserPreferencesNotFound
		}
		return db.UserPreference{}, fmt.Errorf("failed to find user preferences: %w", err)
	}
	return prefs, nil
}

func (c *UserPreferencesController) Update(ctx context.Context, q db.Querier, userID string, params UpdatePreferencesParams) (db.UserPreference, error) {
	translateContent := int64(0)
	if params.TranslateContent {
		translateContent = 1
	}

	prefs, err := q.UpdateUserPreferences(ctx, db.UpdateUserPreferencesParams{
		Theme:            string(params.Theme),
		Language:         string(params.Language),
		TranslateContent: translateContent,
		AiPersonality:    string(params.AIPersonality),
		UserID:           userID,
	})
	if err != nil {
		return db.UserPreference{}, fmt.Errorf("failed to update user preferences: %w", err)
	}
	return prefs, nil
}

func (c *UserPreferencesController) SoftDelete(ctx context.Context, q db.Querier, userID string) error {
	if err := q.SoftDeleteUserPreferences(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user preferences: %w", err)
	}
	return nil
}
