package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
)

type RefreshTokenController struct {
	expiry time.Duration
}

func NewRefreshTokenController(expiry time.Duration) *RefreshTokenController {
	return &RefreshTokenController{expiry: expiry}
}

func (c *RefreshTokenController) Create(ctx context.Context, q db.Querier, userID string) (db.RefreshToken, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.RefreshToken{}, fmt.Errorf("failed to generate refresh token ID: %w", err)
	}

	token, err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID:        id.String(),
		UserID:    userID,
		ExpiresAt: utctime.New(time.Now().Add(c.expiry)),
	})
	if err != nil {
		return db.RefreshToken{}, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return token, nil
}

func (c *RefreshTokenController) FindByID(ctx context.Context, q db.Querier, id string) (db.RefreshToken, error) {
	token, err := q.FindRefreshTokenByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.RefreshToken{}, ErrRefreshTokenNotFound
		}
		return db.RefreshToken{}, fmt.Errorf("failed to find refresh token: %w", err)
	}

	if time.Now().After(token.ExpiresAt.Time) {
		return db.RefreshToken{}, ErrRefreshTokenExpired
	}

	return token, nil
}

func (c *RefreshTokenController) Extend(ctx context.Context, q db.Querier, id string) error {
	if err := q.ExtendRefreshToken(ctx, db.ExtendRefreshTokenParams{
		ID:        id,
		ExpiresAt: utctime.New(time.Now().Add(c.expiry)),
	}); err != nil {
		return fmt.Errorf("failed to extend refresh token: %w", err)
	}
	return nil
}

func (c *RefreshTokenController) Revoke(ctx context.Context, q db.Querier, id string) error {
	if err := q.RevokeRefreshToken(ctx, id); err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (c *RefreshTokenController) RevokeAll(ctx context.Context, q db.Querier, userID string) error {
	if err := q.RevokeAllRefreshTokensByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke all refresh tokens: %w", err)
	}
	return nil
}
