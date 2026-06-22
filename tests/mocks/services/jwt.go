package services

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

const TestJWTSecret = "test-secret-key-for-unit-tests-only"

func GenerateTestAccessToken(user db.User, refreshTokenID string) (string, error) {
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
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(TestJWTSecret))
}

func GenerateExpiredTestAccessToken(user db.User, refreshTokenID string) (string, error) {
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
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(TestJWTSecret))
}
