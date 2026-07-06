package utils

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/oauthstate"
)

// TestOAuthRedirectURI is the client callback URL tests use as the OAuth redirect target. Any URI
// works (the callback trusts the signed state's redirect_uri, not an allowlist), so tests fix one.
const TestOAuthRedirectURI = "http://localhost:5173/auth/callback"

// CompleteOAuthLogin drives the login callback the way the real Google round-trip would: it mints a
// valid signed state for TestOAuthRedirectURI (as GET /auth/google does), calls the callback with a
// code, and parses the tokens the callback returns in the redirect fragment. It returns the access
// token and the refresh token id. Used by the non-auth E2E flows that just need an authenticated
// session; the auth suite exercises the full two-step flow directly.
func CompleteOAuthLogin(t *testing.T, app *fiber.App, secret []byte) (accessToken, refreshTokenID string) {
	t.Helper()

	state, err := oauthstate.Generate(secret, TestOAuthRedirectURI)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=any-code&state="+url.QueryEscape(state), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)

	return ParseTokensFromRedirect(t, resp.Header.Get("Location"))
}

// ParseTokensFromRedirect extracts access_token and refresh_token from the callback's redirect
// Location fragment (redirect_uri#access_token=...&refresh_token=...&expires_in=...).
func ParseTokensFromRedirect(t *testing.T, location string) (accessToken, refreshTokenID string) {
	t.Helper()
	require.NotEmpty(t, location, "expected a redirect Location")

	hash := strings.IndexByte(location, '#')
	require.GreaterOrEqual(t, hash, 0, "expected a fragment in %q", location)

	values, err := url.ParseQuery(location[hash+1:])
	require.NoError(t, err)

	access := values.Get("access_token")
	refresh := values.Get("refresh_token")
	require.NotEmpty(t, access, "access_token missing from redirect fragment")
	require.NotEmpty(t, refresh, "refresh_token missing from redirect fragment")
	return access, refresh
}
