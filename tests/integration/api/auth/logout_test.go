package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

func TestLogout_Integration_RevokesRefreshToken(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "google-logout-1", Email: "logout@example.com", Name: "Logout User",
		},
	}
	app, _, _ := setupIntegrationApp(t, oauth)

	accessToken, refreshToken := loginAndGetTokens(t, app)

	// Logout
	logoutReq, err := http.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	require.NoError(t, err)
	logoutReq.Header.Set("Authorization", "Bearer "+accessToken)

	logoutResp, err := app.Test(logoutReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, logoutResp.StatusCode)

	// Refresh after logout should fail since refresh_token is revoked
	refreshReq, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
	require.NoError(t, err)
	refreshReq.Header.Set("Content-Type", "application/json")

	refreshResp, err := app.Test(refreshReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode)
}

func TestLogout_Integration_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
