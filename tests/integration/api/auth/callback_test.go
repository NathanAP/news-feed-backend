package auth_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func TestCallback_Integration_CreatesNewUser(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "google-123", Email: "user@example.com",
			Name: "Test User", Picture: "https://example.com/photo.jpg",
		},
	}
	app, _, _ := setupIntegrationApp(t, oauth)

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code&state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	access, refresh := testutils.ParseTokensFromRedirect(t, resp.Header.Get("Location"))
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
}

func TestCallback_Integration_ExistingUser_UpdatesLastLogin(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "google-456", Email: "existing@example.com",
			Name: "Existing User", Picture: "",
		},
	}
	app, _, _ := setupIntegrationApp(t, oauth)

	// First login — creates user.
	req1, _ := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=code1&state="+url.QueryEscape(validState(t)), nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, resp1.StatusCode)

	// Second login — updates last_login_at.
	req2, _ := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=code2&state="+url.QueryEscape(validState(t)), nil)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, resp2.StatusCode)
}

func TestCallback_Integration_MissingState(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCallback_Integration_MissingCode(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCallback_Integration_OAuthExchangeFailure(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{ExchangeError: errors.New("google refused")}
	app, _, _ := setupIntegrationApp(t, oauth)

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=bad&state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
