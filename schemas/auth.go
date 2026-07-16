package schemas

import (
	"github.com/golang-jwt/jwt/v5"

	"github.com/nathanap/news-feed-backend/schemas/enums"
	"github.com/nathanap/news-feed-backend/services/utctime"
)

type Claims struct {
	UserID         string `json:"user_id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Picture        string `json:"picture"`
	RefreshTokenID string `json:"refresh_token_id"`
	// utctime.Time, not time.Time: this is a wire value the client reads out of the token payload, so
	// it must serialize as UTC no matter which host issued it. The other response schemas can stay on
	// time.Time because the value they carry has already been normalized when scanned from the DB;
	// here the type is what keeps the token itself honest.
	CreatedAt           utctime.Time        `json:"created_at"`
	LanguageToTranslate *enums.Language     `json:"language_to_translate"`
	AIPersonality       enums.AIPersonality `json:"ai_personality"`
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
