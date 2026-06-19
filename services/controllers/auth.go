package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type AuthController struct {
	oauth2Config      *oauth2.Config
	userCtrl          *UserController
	refreshTokenCtrl  *RefreshTokenController
	jwtSecret         []byte
	accessTokenExpiry time.Duration
}

func NewAuthController(
	oauth2Config *oauth2.Config,
	userCtrl *UserController,
	refreshTokenCtrl *RefreshTokenController,
	jwtSecret []byte,
	accessTokenExpiry time.Duration,
) *AuthController {
	return &AuthController{
		oauth2Config:      oauth2Config,
		userCtrl:          userCtrl,
		refreshTokenCtrl:  refreshTokenCtrl,
		jwtSecret:         jwtSecret,
		accessTokenExpiry: accessTokenExpiry,
	}
}

func (c *AuthController) HandleGoogleCallback(ctx context.Context, code string) (schemas.AuthResponse, error) {
	googleToken, err := c.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return schemas.AuthResponse{}, fmt.Errorf("failed to exchange oauth2 code: %w", err)
	}

	userInfo, err := c.fetchGoogleUserInfo(ctx, googleToken)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	user, err := c.findOrCreateUser(ctx, userInfo)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	if err := c.refreshTokenCtrl.RevokeAll(ctx, user.ID); err != nil {
		return schemas.AuthResponse{}, err
	}

	if err := c.userCtrl.UpdateUserLastLogin(ctx, user.ID); err != nil {
		return schemas.AuthResponse{}, err
	}

	refreshToken, err := c.refreshTokenCtrl.Create(ctx, user.ID)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	accessToken, err := c.GenerateAccessToken(user, refreshToken.ID)
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
	refreshToken, err := c.refreshTokenCtrl.FindByID(ctx, refreshTokenID)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	user, err := c.userCtrl.FindUserByID(ctx, refreshToken.UserID)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	if err := c.refreshTokenCtrl.Extend(ctx, refreshToken.ID); err != nil {
		return schemas.AuthResponse{}, err
	}

	accessToken, err := c.GenerateAccessToken(user, refreshToken.ID)
	if err != nil {
		return schemas.AuthResponse{}, err
	}

	return schemas.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken.ID,
		ExpiresIn:    int(c.accessTokenExpiry.Seconds()),
	}, nil
}

func (c *AuthController) GenerateAccessToken(user db.User, refreshTokenID string) (string, error) {
	picture := ""
	if user.Picture.Valid {
		picture = user.Picture.String
	}

	claims := schemas.Claims{
		UserID:         user.ID,
		Email:          user.Email,
		Name:           user.Name,
		Picture:        picture,
		RefreshTokenID: refreshTokenID,
		CreatedAt:      user.CreatedAt,
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

func (c *AuthController) fetchGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*googleUserInfo, error) {
	client := c.oauth2Config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
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

func (c *AuthController) findOrCreateUser(ctx context.Context, info *googleUserInfo) (db.User, error) {
	user, err := c.userCtrl.FindUserByGoogleID(ctx, info.ID)
	if err == nil {
		return user, nil
	}

	if err != ErrUserNotFound {
		return db.User{}, err
	}

	return c.userCtrl.CreateUser(ctx, info.ID, info.Email, info.Name, info.Picture)
}
