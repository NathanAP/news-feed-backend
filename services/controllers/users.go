package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

type UserController struct{}

func NewUserController() *UserController {
	return &UserController{}
}

// CreateUser creates a user and its default preferences on the given querier without
// committing. The caller is responsible for the transaction boundary; running both
// writes inside a single transaction prevents orphan users (a user without preferences
// would be unable to log in, since the login flow needs preferences to build the token).
func (c *UserController) CreateUser(ctx context.Context, q db.Querier, googleID, email, name, picture string) (db.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.User{}, fmt.Errorf("failed to generate user ID: %w", err)
	}

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		ID:       id.String(),
		GoogleID: googleID,
		Email:    email,
		Name:     name,
		Picture:  sql.NullString{String: picture, Valid: picture != ""},
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return db.User{}, ErrUserAlreadyExists
		}
		return db.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	prefParams, err := DefaultPreferencesParams(user.ID)
	if err != nil {
		return db.User{}, err
	}
	if _, err := q.CreateUserPreferences(ctx, prefParams); err != nil {
		return db.User{}, fmt.Errorf("failed to create user preferences: %w", err)
	}

	return user, nil
}

func (c *UserController) FindUserByID(ctx context.Context, q db.Querier, id string) (db.User, error) {
	user, err := q.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("failed to find user: %w", err)
	}

	return user, nil
}

func (c *UserController) FindUserByGoogleID(ctx context.Context, q db.Querier, googleID string) (db.User, error) {
	user, err := q.FindUserByGoogleID(ctx, googleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("failed to find user: %w", err)
	}

	return user, nil
}

func (c *UserController) UpdateUserLastLogin(ctx context.Context, q db.Querier, id string) error {
	if err := q.UpdateUserLastLogin(ctx, id); err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

// SoftDeleteUser soft-deletes the user, soft-deletes its preferences and revokes all its
// refresh tokens on the given querier without committing. Running them together (in the
// caller's transaction) guarantees a deleted user can no longer authenticate or refresh.
func (c *UserController) SoftDeleteUser(ctx context.Context, q db.Querier, id string) error {
	if err := q.SoftDeleteUser(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if err := q.SoftDeleteUserPreferences(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user preferences: %w", err)
	}

	if err := q.RevokeAllRefreshTokensByUserID(ctx, id); err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	return nil
}
