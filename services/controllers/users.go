package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		if isUniqueViolation(err) {
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

// SetAdmin promotes or demotes a user on the given querier without committing. It is the only way an
// administrator comes into existence: CreateUser does not touch the column, so no login path can
// produce one by accident. Only the development seed calls it today (PROJECT.md keeps promotion a
// manual act until an admin-management endpoint exists).
//
// A soft-removed user cannot be promoted — the query filters on the active predicate like every other
// lookup, so a missing row means "no active user with that id" rather than a silent no-op.
func (c *UserController) SetAdmin(ctx context.Context, q db.Querier, id string, admin bool) (db.User, error) {
	user, err := q.SetUserAdmin(ctx, db.SetUserAdminParams{ID: id, Admin: admin})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("failed to set user admin flag: %w", err)
	}

	return user, nil
}

// SoftDeleteUser soft-deletes the user and cascades the soft-delete to its preferences,
// refresh tokens and feeds on the given querier without committing. Running them together
// (in the caller's transaction) guarantees a deleted user can no longer authenticate,
// refresh, or keep owning feeds. There is no endpoint exposing this yet — user removal is a
// future feature — but the cascade is kept correct for when one lands.
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

	if err := q.SoftDeleteFeedsByUser(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user feeds: %w", err)
	}

	return nil
}
