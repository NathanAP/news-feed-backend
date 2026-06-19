package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
)

type RefreshTokenController struct {
	queries *db.Queries
	expiry  time.Duration
}

func NewRefreshTokenController(database *sql.DB, expiry time.Duration) *RefreshTokenController {
	return &RefreshTokenController{
		queries: db.New(database),
		expiry:  expiry,
	}
}

func (c *RefreshTokenController) Create(ctx context.Context, userID string) (db.RefreshToken, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.RefreshToken{}, fmt.Errorf("failed to generate refresh token ID: %w", err)
	}

	params := db.CreateRefreshTokenParams{
		ID:        id.String(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(c.expiry),
	}

	token, err := c.queries.CreateRefreshToken(ctx, params)
	if err != nil {
		return db.RefreshToken{}, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return token, nil
}

func (c *RefreshTokenController) FindByID(ctx context.Context, id string) (db.RefreshToken, error) {
	token, err := c.queries.FindRefreshTokenByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.RefreshToken{}, ErrRefreshTokenNotFound
		}
		return db.RefreshToken{}, fmt.Errorf("failed to find refresh token: %w", err)
	}

	if time.Now().After(token.ExpiresAt) {
		return db.RefreshToken{}, ErrRefreshTokenExpired
	}

	return token, nil
}

func (c *RefreshTokenController) Extend(ctx context.Context, id string) error {
	params := db.ExtendRefreshTokenParams{
		ID:        id,
		ExpiresAt: time.Now().Add(c.expiry),
	}
	if err := c.queries.ExtendRefreshToken(ctx, params); err != nil {
		return fmt.Errorf("failed to extend refresh token: %w", err)
	}
	return nil
}

func (c *RefreshTokenController) Revoke(ctx context.Context, id string) error {
	if err := c.queries.RevokeRefreshToken(ctx, id); err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (c *RefreshTokenController) RevokeAll(ctx context.Context, userID string) error {
	if err := c.queries.RevokeAllRefreshTokensByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke all refresh tokens: %w", err)
	}
	return nil
}
