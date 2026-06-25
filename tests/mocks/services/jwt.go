package services

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

const TestJWTSecret = "test-secret-key-for-unit-tests-only"

func GenerateTestAccessToken(user db.User, refreshTokenID string, prefs ...db.UserPreference) (string, error) {
	picture := ""
	if user.Picture.Valid {
		picture = user.Picture.String
	}

	theme := schemas.ThemeDark
	language := schemas.LanguagePT
	translateContent := true
	aiPersonality := schemas.AIPersonalityMixed

	if len(prefs) > 0 {
		theme = schemas.Theme(prefs[0].Theme)
		language = schemas.Language(prefs[0].Language)
		translateContent = prefs[0].TranslateContent == 1
		aiPersonality = schemas.AIPersonality(prefs[0].AiPersonality)
	}

	claims := schemas.Claims{
		UserID:           user.ID,
		Email:            user.Email,
		Name:             user.Name,
		Picture:          picture,
		RefreshTokenID:   refreshTokenID,
		CreatedAt:        user.CreatedAt,
		Theme:            theme,
		Language:         language,
		TranslateContent: translateContent,
		AIPersonality:    aiPersonality,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(TestJWTSecret))
}
