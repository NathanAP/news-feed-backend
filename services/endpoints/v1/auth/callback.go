package auth

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/oauthstate"
)

// GoogleCallback finishes the OAuth login. It first validates the signed state (401 on failure — a
// forged or stale callback), recovering the trusted redirect_uri. If Google reported an error (the
// user cancelled consent), it bounces back to the client with ?error=. Otherwise it exchanges the
// code for the user, mints our tokens, and redirects to the client's redirect_uri with the tokens in
// the URL fragment — which never reaches a server or log. The client reads and clears the fragment,
// then uses the access token as a Bearer header.
func GoogleCallback(authCtrl controllers.AuthControllerInterface, stateSecret []byte) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		redirectURI, err := oauthstate.Validate(stateSecret, c.Query("state"))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid state"})
		}

		// The user denied consent (or Google returned an error): bounce back so the client can show
		// a friendly message instead of a dead end on the API domain.
		if oauthErr := c.Query("error"); oauthErr != "" {
			return c.Redirect(appendQueryParam(redirectURI, "error", oauthErr), fiber.StatusTemporaryRedirect)
		}

		code := c.Query("code")
		if code == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authorization code is required"})
		}

		authResponse, err := authCtrl.HandleGoogleCallback(c.Context(), code)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
		}

		// Tokens ride back in the fragment (not the query): the fragment is never sent to a server,
		// keeping them out of access logs and Referer headers.
		fragment := "access_token=" + url.QueryEscape(authResponse.AccessToken) +
			"&refresh_token=" + url.QueryEscape(authResponse.RefreshToken) +
			"&expires_in=" + strconv.Itoa(authResponse.ExpiresIn)

		return c.Redirect(redirectURI+"#"+fragment, fiber.StatusTemporaryRedirect)
	}
}

// appendQueryParam appends key=value to base's query string, choosing ? or & depending on whether
// base already has a query.
func appendQueryParam(base, key, value string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + key + "=" + url.QueryEscape(value)
}
