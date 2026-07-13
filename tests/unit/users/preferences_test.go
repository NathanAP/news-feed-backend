package users_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/fixtures"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
)

func buildAuthToken(t *testing.T) string {
	t.Helper()
	user := fixtures.NewTestUser()
	rt := fixtures.NewTestRefreshToken(user.ID)
	token, err := jwtmock.GenerateTestAccessToken(user, rt.ID)
	require.NoError(t, err)
	return token
}

// ── GET /v1/users/me/preferences ─────────────────────────────────────────────

func TestGetPreferences_Success(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+buildAuthToken(t))

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, readJSON(resp, &body))
	assert.NotEmpty(t, body["language_to_translate"])
	assert.NotEmpty(t, body["ai_personality"])
	// theme and translate_content were removed in 0.33.
	_, hasTheme := body["theme"]
	assert.False(t, hasTheme)
}

func TestGetPreferences_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	req, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── PUT /v1/users/me/preferences ─────────────────────────────────────────────

func TestUpdatePreferences_Success(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	body := `{"language_to_translate":"en","ai_personality":"fun"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+buildAuthToken(t))

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, readJSON(resp, &result))
	assert.NotEmpty(t, result["access_token"])
	assert.NotNil(t, result["preferences"])
}

func TestUpdatePreferences_NullLanguageDisablesTranslation(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	body := `{"language_to_translate":null,"ai_personality":"mixed"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+buildAuthToken(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUpdatePreferences_InvalidLanguageToTranslate(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	body := `{"language_to_translate":"klingon","ai_personality":"mixed"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+buildAuthToken(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePreferences_InvalidAIPersonality(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	body := `{"language_to_translate":"pt","ai_personality":"chaotic"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+buildAuthToken(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePreferences_MissingBody(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+buildAuthToken(t))

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePreferences_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app := setupPreferencesApp()
	body := `{"language_to_translate":"en","ai_personality":"fun"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
