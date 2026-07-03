package schemas

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/nathanap/news-feed-backend/schemas/enums"
)

type Claims struct {
	UserID           string              `json:"user_id"`
	Email            string              `json:"email"`
	Name             string              `json:"name"`
	Picture          string              `json:"picture"`
	RefreshTokenID   string              `json:"refresh_token_id"`
	CreatedAt        time.Time           `json:"created_at"`
	Theme            enums.Theme         `json:"theme"`
	Language         enums.Language      `json:"language"`
	TranslateContent bool                `json:"translate_content"`
	AIPersonality    enums.AIPersonality `json:"ai_personality"`
	jwt.RegisteredClaims
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}
