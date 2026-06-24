package controllers

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type OAuth2Provider interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, t *oauth2.Token) *http.Client
}

type UserControllerInterface interface {
	CreateUser(ctx context.Context, googleID, email, name, picture string) (db.User, error)
	FindUserByID(ctx context.Context, id string) (db.User, error)
	FindUserByGoogleID(ctx context.Context, googleID string) (db.User, error)
	UpdateUserLastLogin(ctx context.Context, id string) error
	SoftDeleteUser(ctx context.Context, id string) error
}

type UserPreferencesControllerInterface interface {
	CreateDefault(ctx context.Context, userID string) (db.UserPreference, error)
	FindByUserID(ctx context.Context, userID string) (db.UserPreference, error)
	Update(ctx context.Context, userID string, params UpdatePreferencesParams) (db.UserPreference, error)
	SoftDelete(ctx context.Context, userID string) error
}

type RefreshTokenControllerInterface interface {
	Create(ctx context.Context, userID string) (db.RefreshToken, error)
	FindByID(ctx context.Context, id string) (db.RefreshToken, error)
	Extend(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
	RevokeAll(ctx context.Context, userID string) error
}

type AuthControllerInterface interface {
	HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error)
	RefreshAccessToken(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error)
	GenerateAccessToken(user db.User, refreshTokenID string, prefs db.UserPreference) (string, error)
	RegenerateFromClaims(ctx context.Context, claims *schemas.Claims, updatedPrefs db.UserPreference) (string, error)
}
