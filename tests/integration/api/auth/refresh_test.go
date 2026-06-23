package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

func TestRefresh_Integration_ReturnsNewAccessToken(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "google-refresh-1", Email: "refresh@example.com", Name: "Refresh User",
		},
	}
	app, _, _ := setupIntegrationApp(t, oauth)

	_, refreshToken := loginAndGetTokens(t, app)

	body := `{"refresh_token":"` + refreshToken + `"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["access_token"])
	assert.NotEmpty(t, result["refresh_token"])
}

func TestRefresh_Integration_UnknownToken(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{}
	app, _, _ := setupIntegrationApp(t, oauth)

	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"00000000-0000-0000-0000-000000000000"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRefresh_Integration_MissingBody(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
