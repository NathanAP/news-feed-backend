package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

func TestInvalidateAll_Integration_RevokesAllUserTokens(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "google-invall-1", Email: "invall@example.com", Name: "InvAll User",
		},
	}
	app, _, _ := setupIntegrationApp(t, oauth)

	// Login to establish a session
	_, refreshToken := loginAndGetTokens(t, app)

	// Invalidate all sessions — we need the user_id.
	// Get it by decoding the refresh_token's user via a second login that returns a new token.
	// We'll use the invalidate-all with query param approach.
	// Since we don't have a direct /me query without access_token, use the access_token from first login.
	accessToken, _ := loginAndGetTokens(t, app)

	// Get user_id from the access_token (it's a JWT — decode the sub/user_id)
	userID := extractUserIDFromToken(t, accessToken)

	req, err := http.NewRequest(http.MethodDelete,
		"/v1/auth/invalidate-all?user_id="+userID, nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// The refresh token from the first login should now be revoked
	refreshReq, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
	require.NoError(t, err)
	refreshReq.Header.Set("Content-Type", "application/json")

	refreshResp, err := app.Test(refreshReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode)
}

func TestInvalidateAll_Integration_MissingUserID(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate-all", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
