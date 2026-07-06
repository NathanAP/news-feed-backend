package auth

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/oauthstate"
)

// GoogleLogin starts the OAuth login. The client passes its redirect_uri (where the callback should
// return the user); it must exactly match one of the allowlisted URIs — otherwise 400, and we never
// redirect (an unvalidated redirect target is an open-redirect / token-exfiltration vector). The
// validated redirect_uri is bound into a signed state (CSRF, and the vehicle that carries it through
// the Google round-trip), and we redirect to Google's consent screen with our fixed callback.
func GoogleLogin(cfg *oauth2.Config, stateSecret []byte, allowedRedirectURIs []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		redirectURI := c.Query("redirect_uri")
		if !isAllowedRedirectURI(redirectURI, allowedRedirectURIs) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid redirect_uri"})
		}

		state, err := oauthstate.Generate(stateSecret, redirectURI)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
		}

		return c.Redirect(cfg.AuthCodeURL(state), fiber.StatusTemporaryRedirect)
	}
}

// isAllowedRedirectURI reports whether uri exactly matches one of the allowlisted redirect URIs.
// Exact match only — never prefix/substring, which is bypassable (https://app.com.evil.com starts
// with https://app.com).
func isAllowedRedirectURI(uri string, allowed []string) bool {
	if uri == "" {
		return false
	}
	for _, a := range allowed {
		if uri == a {
			return true
		}
	}
	return false
}
