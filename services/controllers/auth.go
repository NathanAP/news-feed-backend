package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type AuthController struct {
	oauth2Provider    OAuth2Provider
	userCtrl          UserControllerInterface
	refreshTokenCtrl  RefreshTokenControllerInterface
	prefCtrl          UserPreferencesControllerInterface
	runTx             TransactionRunner
	jwtSecret         []byte
	accessTokenExpiry time.Duration
}

func NewAuthController(
	oauth2Provider OAuth2Provider,
	userCtrl UserControllerInterface,
	refreshTokenCtrl RefreshTokenControllerInterface,
	prefCtrl UserPreferencesControllerInterface,
	runTx TransactionRunner,
	jwtSecret []byte,
	accessTokenExpiry time.Duration,
) *AuthController {
	return &AuthController{
		oauth2Provider:    oauth2Provider,
		userCtrl:          userCtrl,
		refreshTokenCtrl:  refreshTokenCtrl,
		prefCtrl:          prefCtrl,
		runTx:             runTx,
		jwtSecret:         jwtSecret,
		accessTokenExpiry: accessTokenExpiry,
	}
}

func (c *AuthController) HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error) {
	googleToken, err := c.oauth2Provider.Exchange(ctx, code)
	if err != nil {
		return schemas.AuthResponse{}, fmt.Errorf("failed to exchange oauth2 code: %w", err)
	}

	userInfo, err := c.fetchGoogleUserInfo(ctx, googleToken)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	var (
		user         db.User
		prefs        db.UserPreference
		refreshToken db.RefreshToken
	)

	err = c.runTx(ctx, func(q db.Querier) error {
		user, err = c.findOrCreateUser(ctx, q, userInfo)
		if err != nil {
			return err
		}

		if err := c.refreshTokenCtrl.RevokeAll(ctx, q, user.ID); err != nil {
			return err
		}

		if err := c.userCtrl.UpdateUserLastLogin(ctx, q, user.ID); err != nil {
			return err
		}

		prefs, err = c.prefCtrl.FindByUserID(ctx, q, user.ID)
		if err != nil {
			return err
		}

		refreshToken, err = c.refreshTokenCtrl.Create(ctx, q, user.ID)
		return err
	})
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	accessToken, err := c.GenerateAccessToken(user, refreshToken.ID, prefs)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	return schemas.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken.ID,
		ExpiresIn:    int(c.accessTokenExpiry.Seconds()),
	}, nil
}

func (c *AuthController) RefreshAccessToken(ctx context.Context, refreshTokenID string) (schemas.AuthResponse, error) {
	var (
		user         db.User
		prefs        db.UserPreference
		refreshToken db.RefreshToken
	)

	err := c.runTx(ctx, func(q db.Querier) error {
		var err error
		refreshToken, err = c.refreshTokenCtrl.FindByID(ctx, q, refreshTokenID)
		if err != nil {
			return err
		}

		user, err = c.userCtrl.FindUserByID(ctx, q, refreshToken.UserID)
		if err != nil {
			return err
		}

		prefs, err = c.prefCtrl.FindByUserID(ctx, q, user.ID)
		if err != nil {
			return err
		}

		return c.refreshTokenCtrl.Extend(ctx, q, refreshToken.ID)
	})
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	accessToken, err := c.GenerateAccessToken(user, refreshToken.ID, prefs)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	return schemas.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken.ID,
		ExpiresIn:    int(c.accessTokenExpiry.Seconds()),
	}, nil
}

func (c *AuthController) GenerateAccessToken(user db.User, refreshTokenID string, prefs db.UserPreference) (string, error) {
	picture := ""
	if user.Picture.Valid {
		picture = user.Picture.String
	}

	claims := schemas.Claims{
		UserID:           user.ID,
		Email:            user.Email,
		Name:             user.Name,
		Picture:          picture,
		RefreshTokenID:   refreshTokenID,
		CreatedAt:        user.CreatedAt,
		Theme:            enums.Theme(prefs.Theme),
		Language:         enums.Language(prefs.Language),
		TranslateContent: prefs.TranslateContent == 1,
		AIPersonality:    enums.AIPersonality(prefs.AiPersonality),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(c.accessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(c.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}

	return signed, nil
}

func (c *AuthController) RegenerateFromClaims(ctx context.Context, q db.Querier, claims *schemas.Claims, updatedPrefs db.UserPreference) (string, error) {
	user, err := c.userCtrl.FindUserByID(ctx, q, claims.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user for token regeneration: %w", err)
	}
	return c.GenerateAccessToken(user, claims.RefreshTokenID, updatedPrefs)
}

func (c *AuthController) fetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*googleUserInfo, error) {
	httpClient := c.oauth2Provider.Client(ctx, token)

	resp, err := httpClient.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch google user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read google user info response: %w", err)
	}

	var userInfo googleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse google user info: %w", err)
	}

	return &userInfo, nil
}

func (c *AuthController) findOrCreateUser(ctx context.Context, q db.Querier, info *googleUserInfo) (db.User, error) {
	user, err := c.userCtrl.FindUserByGoogleID(ctx, q, info.ID)
	if err == nil {
		return user, nil
	}

	if !errors.Is(err, ErrUserNotFound) {
		return db.User{}, err
	}

	return c.userCtrl.CreateUser(ctx, q, info.ID, info.Email, info.Name, info.Picture)
}
