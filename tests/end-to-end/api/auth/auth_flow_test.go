package auth_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

// ── OAuth2 callback scenarios ────────────────────────────────────────────────

func TestE2E_Callback_SuccessfulLogin(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-google-1", Email: "e2e@example.com",
			Name: "E2E User", Picture: "https://example.com/pic.jpg",
		},
	}
	app, queries, _ := setupE2EApp(t, oauth)

	user, rt, accessToken := loginViaCallback(t, app, queries)

	// Verify the persisted data matches what the mock returned
	assert.Equal(t, "e2e@example.com", user.Email)
	assert.Equal(t, "E2E User", user.Name)
	assert.True(t, user.Picture.Valid)
	assert.Equal(t, "https://example.com/pic.jpg", user.Picture.String)
	assert.Equal(t, int64(1), user.Status)
	assert.Equal(t, user.ID, rt.UserID)
	assert.Equal(t, int64(1), rt.Status)
	assert.NotEmpty(t, accessToken)
}

func TestE2E_Callback_MissingCode(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	// Valid state but no code → authentication cannot proceed (401).
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_Callback_MissingState(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=any-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_Callback_OAuthFailure(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{
		ExchangeError: assert.AnError,
	})

	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=bad&state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── Full auth flow ───────────────────────────────────────────────────────────

func TestE2E_FullFlow_LoginRefreshLogout(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-flow-1", Email: "flow@example.com", Name: "Flow User",
		},
	}
	app, queries, _ := setupE2EApp(t, oauth)

	// Establish an authenticated session through the real login flow
	_, rt, accessToken := loginViaCallback(t, app, queries)
	refreshTokenID := rt.ID

	// /me works with valid token
	meReq, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	meReq.Header.Set("Authorization", "Bearer "+accessToken)

	meResp, err := app.Test(meReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, meResp.StatusCode)
	readJSON(meResp, &map[string]interface{}{})

	// Refresh access token
	refreshReq, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"`+refreshTokenID+`"}`))
	require.NoError(t, err)
	refreshReq.Header.Set("Content-Type", "application/json")

	refreshResp, err := app.Test(refreshReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, refreshResp.StatusCode)

	var refreshBody map[string]interface{}
	require.NoError(t, readJSON(refreshResp, &refreshBody))
	newAccessToken := refreshBody["access_token"].(string)
	assert.NotEmpty(t, newAccessToken)

	// Logout with new token
	logoutReq, err := http.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	require.NoError(t, err)
	logoutReq.Header.Set("Authorization", "Bearer "+newAccessToken)

	logoutResp, err := app.Test(logoutReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, logoutResp.StatusCode)

	// /me fails after logout (session revoked)
	afterLogoutReq, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	afterLogoutReq.Header.Set("Authorization", "Bearer "+newAccessToken)

	afterLogoutResp, err := app.Test(afterLogoutReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, afterLogoutResp.StatusCode)
}

func TestE2E_FullFlow_SingleSessionPolicy(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-single-1", Email: "single@example.com", Name: "Single User",
		},
	}
	app, queries, _ := setupE2EApp(t, oauth)

	// First login through the real flow.
	_, firstRT, _ := loginViaCallback(t, app, queries)
	firstRefreshToken := firstRT.ID

	// Second login — invalidates the first session.
	loginViaCallback(t, app, queries)

	// First refresh token should now be invalid
	refreshReq, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"`+firstRefreshToken+`"}`))
	require.NoError(t, err)
	refreshReq.Header.Set("Content-Type", "application/json")

	refreshResp, err := app.Test(refreshReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode)
}

// ── Authentication guard scenarios ──────────────────────────────────────────

func TestE2E_ProtectedRoute_NoToken(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_ProtectedRoute_InvalidToken(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not.a.real.jwt")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_ProtectedRoute_MalformedAuthHeader(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "NotBearer token-here")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_RouteNotFound(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/route-does-not-exist", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── Refresh token scenarios ──────────────────────────────────────────────────

func TestE2E_Refresh_MissingBody(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Refresh_InvalidToken(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodPost, "/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"00000000-0000-0000-0000-000000000000"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── Invalidate scenarios ─────────────────────────────────────────────────────

func TestE2E_InvalidateAll_MissingUserID(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate-all", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Invalidate_MissingBody(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
