package services

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/schemas/enums"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

const TestJWTSecret = "test-secret-key-for-unit-tests-only"

func GenerateTestAccessToken(user db.User, refreshTokenID string, prefs ...db.UserPreference) (string, error) {
	picture := ""
	if user.Picture.Valid {
		picture = user.Picture.String
	}

	defaultLang := enums.LanguagePT
	languageToTranslate := &defaultLang
	aiPersonality := enums.AIPersonalityMixed

	if len(prefs) > 0 {
		if prefs[0].LanguageToTranslate.Valid {
			lang := enums.Language(prefs[0].LanguageToTranslate.String)
			languageToTranslate = &lang
		} else {
			languageToTranslate = nil
		}
		aiPersonality = enums.AIPersonality(prefs[0].AiPersonality)
	}

	claims := schemas.Claims{
		UserID:              user.ID,
		Email:               user.Email,
		Name:                user.Name,
		Picture:             picture,
		RefreshTokenID:      refreshTokenID,
		CreatedAt:           user.CreatedAt,
		LanguageToTranslate: languageToTranslate,
		AIPersonality:       aiPersonality,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(TestJWTSecret))
}
