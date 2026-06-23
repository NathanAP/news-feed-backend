package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

func TestInvalidate_Integration_RevokesSpecificToken(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "google-inv-1", Email: "inv@example.com", Name: "Inv User",
		},
	}
	app, _, _ := setupIntegrationApp(t, oauth)

	_, refreshToken := loginAndGetTokens(t, app)

	body := `{"refresh_token_id":"` + refreshToken + `"}`
	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Attempting refresh with the revoked token should fail
	refreshReq, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"`+refreshToken+`"}`))
	require.NoError(t, err)
	refreshReq.Header.Set("Content-Type", "application/json")

	refreshResp, err := app.Test(refreshReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode)
}

func TestInvalidate_Integration_MissingID(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupIntegrationApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate",
		strings.NewReader(`{}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
