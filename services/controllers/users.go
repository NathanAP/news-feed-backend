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

type UserController struct {
	queries *db.Queries
}

func NewUserController(database *sql.DB) *UserController {
	return &UserController{
		queries: db.New(database),
	}
}

func (c *UserController) CreateUser(ctx context.Context, googleID, email, name, picture string) (db.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.User{}, fmt.Errorf("failed to generate user ID: %w", err)
	}

	params := db.CreateUserParams{
		ID:       id.String(),
		GoogleID: googleID,
		Email:    email,
		Name:     name,
		Picture:  sql.NullString{String: picture, Valid: picture != ""},
	}

	user, err := c.queries.CreateUser(ctx, params)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return db.User{}, ErrUserAlreadyExists
		}
		return db.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (c *UserController) FindUserByID(ctx context.Context, id string) (db.User, error) {
	user, err := c.queries.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("failed to find user: %w", err)
	}

	return user, nil
}

func (c *UserController) FindUserByGoogleID(ctx context.Context, googleID string) (db.User, error) {
	user, err := c.queries.FindUserByGoogleID(ctx, googleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("failed to find user: %w", err)
	}

	return user, nil
}

func (c *UserController) UpdateUserLastLogin(ctx context.Context, id string) error {
	if err := c.queries.UpdateUserLastLogin(ctx, id); err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

func (c *UserController) SoftDeleteUser(ctx context.Context, id string) error {
	if err := c.queries.SoftDeleteUser(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
