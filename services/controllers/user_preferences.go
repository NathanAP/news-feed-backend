package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var ErrUserPreferencesNotFound = errors.New("user preferences not found")

type UpdatePreferencesParams struct {
	Theme            schemas.Theme
	Language         schemas.Language
	TranslateContent bool
	AIPersonality    schemas.AIPersonality
}

type UserPreferencesController struct {
	queries db.Querier
}

func NewUserPreferencesController(querier db.Querier) *UserPreferencesController {
	return &UserPreferencesController{queries: querier}
}

func (c *UserPreferencesController) CreateDefault(ctx context.Context, userID string) (db.UserPreference, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.UserPreference{}, fmt.Errorf("failed to generate preferences ID: %w", err)
	}

	prefs, err := c.queries.CreateUserPreferences(ctx, db.CreateUserPreferencesParams{
		ID:               id.String(),
		UserID:           userID,
		Theme:            string(schemas.ThemeDark),
		Language:         string(schemas.LanguagePT),
		TranslateContent: 1,
		AiPersonality:    string(schemas.AIPersonalityMixed),
	})
	if err != nil {
		return db.UserPreference{}, fmt.Errorf("failed to create user preferences: %w", err)
	}

	return prefs, nil
}

func (c *UserPreferencesController) FindByUserID(ctx context.Context, userID string) (db.UserPreference, error) {
	prefs, err := c.queries.FindUserPreferencesByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.UserPreference{}, ErrUserPreferencesNotFound
		}
		return db.UserPreference{}, fmt.Errorf("failed to find user preferences: %w", err)
	}
	return prefs, nil
}

func (c *UserPreferencesController) Update(ctx context.Context, userID string, params UpdatePreferencesParams) (db.UserPreference, error) {
	translateContent := int64(0)
	if params.TranslateContent {
		translateContent = 1
	}

	prefs, err := c.queries.UpdateUserPreferences(ctx, db.UpdateUserPreferencesParams{
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

func (c *UserPreferencesController) SoftDelete(ctx context.Context, userID string) error {
	if err := c.queries.SoftDeleteUserPreferences(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user preferences: %w", err)
	}
	return nil
}
