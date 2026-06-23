package auth_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
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

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, readJSON(resp, &body))
	assert.NotEmpty(t, body["access_token"])
	assert.NotEmpty(t, body["refresh_token"])
	assert.NotZero(t, body["expires_in"])
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

	// First login — creates user
	req1, _ := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=code1", nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	readJSON(resp1, &map[string]interface{}{})

	// Second login — updates last_login_at
	req2, _ := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=code2", nil)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestCallback_Integration_MissingCode(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{}
	app, _, _ := setupIntegrationApp(t, oauth)

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCallback_Integration_OAuthExchangeFailure(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{ExchangeError: errors.New("google refused")}
	app, _, _ := setupIntegrationApp(t, oauth)

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=bad", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
