package controllers

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type OAuth2Provider interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	Client(ctx context.Context, t *oauth2.Token) *http.Client
}

type UserControllerInterface interface {
	CreateUser(ctx context.Context, q db.Querier, googleID, email, name, picture string) (db.User, error)
	FindUserByID(ctx context.Context, q db.Querier, id string) (db.User, error)
	FindUserByGoogleID(ctx context.Context, q db.Querier, googleID string) (db.User, error)
	UpdateUserLastLogin(ctx context.Context, q db.Querier, id string) error
	SoftDeleteUser(ctx context.Context, q db.Querier, id string) error
}

type UserPreferencesControllerInterface interface {
	CreateDefault(ctx context.Context, q db.Querier, userID string) (db.UserPreference, error)
	FindByUserID(ctx context.Context, q db.Querier, userID string) (db.UserPreference, error)
	Update(ctx context.Context, q db.Querier, userID string, params UpdatePreferencesParams) (db.UserPreference, error)
	SoftDelete(ctx context.Context, q db.Querier, userID string) error
}

type RefreshTokenControllerInterface interface {
	Create(ctx context.Context, q db.Querier, userID string) (db.RefreshToken, error)
	FindByID(ctx context.Context, q db.Querier, id string) (db.RefreshToken, error)
	Extend(ctx context.Context, q db.Querier, id string) error
	Revoke(ctx context.Context, q db.Querier, id string) error
	RevokeAll(ctx context.Context, q db.Querier, userID string) error
}

type AuthControllerInterface interface {
	HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error)
	RefreshAccessToken(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error)
	GenerateAccessToken(user db.User, refreshTokenID string, prefs db.UserPreference) (string, error)
	RegenerateFromClaims(ctx context.Context, q db.Querier, claims *schemas.Claims, updatedPrefs db.UserPreference) (string, error)
}

type SourceControllerInterface interface {
	Create(ctx context.Context, q db.Querier, url, urlRss string) (db.Source, error)
	FindByID(ctx context.Context, q db.Querier, id string) (db.Source, error)
	List(ctx context.Context, q db.Querier) ([]db.Source, error)
	Update(ctx context.Context, q db.Querier, id, url, urlRss string) (db.Source, error)
	SoftDelete(ctx context.Context, q db.Querier, id string) error
}

type ArticleControllerInterface interface {
	Create(ctx context.Context, q db.Querier, title, content, urlOriginal, sourceID string, keywords []string, languageOriginal *string) (db.Article, error)
	FindByID(ctx context.Context, q db.Querier, id string) (db.Article, error)
	FindByURLOriginal(ctx context.Context, q db.Querier, urlOriginal string) (db.Article, error)
	List(ctx context.Context, q db.Querier) ([]db.Article, error)
	Update(ctx context.Context, q db.Querier, id, title, content, urlOriginal string, keywords []string, languageOriginal *string) (db.Article, error)
	SoftDelete(ctx context.Context, q db.Querier, id string) error
}

type FeedControllerInterface interface {
	Create(ctx context.Context, q db.Querier, userID, name string, keywords []string) (db.Feed, error)
	FindByID(ctx context.Context, q db.Querier, id, userID string) (db.Feed, error)
	FindCandidatesByKeywords(ctx context.Context, q db.Querier, keywords []string) ([]db.Feed, error)
	List(ctx context.Context, q db.Querier, userID string) ([]db.Feed, error)
	Update(ctx context.Context, q db.Querier, id, userID, name string, keywords []string) (db.Feed, error)
	SoftDelete(ctx context.Context, q db.Querier, id, userID string) error
}

type ArticleFeedControllerInterface interface {
	Create(ctx context.Context, q db.Querier, articleID, feedID string) (db.ArticleFeed, error)
	FindByArticleAndUser(ctx context.Context, q db.Querier, articleID, userID string) ([]db.ArticleFeed, error)
	ListArticlesByFeedForUser(ctx context.Context, q db.Querier, feedID, userID string) ([]db.ListArticlesByFeedForUserRow, error)
	MarkAsRead(ctx context.Context, q db.Querier, articleID, userID string) (bool, error)
}

type SystemControllerInterface interface {
	Get(ctx context.Context, q db.Querier) (db.System, error)
	UpdateAppStatus(ctx context.Context, q db.Querier, active bool) (db.System, error)
	UpdateLastArticleDiscovery(ctx context.Context, q db.Querier, at time.Time) (db.System, error)
}
