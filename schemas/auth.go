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
	// Admin is a client hint, never an authorization source. The client reads it to decide whether to
	// render the admin UI; the API always re-reads users.admin from the database before allowing an
	// admin action (middlewares.RequireAdmin), so revoking someone takes effect on the next request
	// instead of whenever their token happens to expire. Same split already used by
	// LanguageToTranslate, which the client acts on but the API re-derives.
	//
	// It follows that a token issued before this field existed decodes as false — a regular user —
	// which is the safe direction, so no session had to be invalidated when 0.40 shipped.
	Admin bool `json:"admin"`
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
