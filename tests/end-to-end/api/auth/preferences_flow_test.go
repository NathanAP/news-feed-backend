package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/tests/mocks/external"
)

func TestE2E_Preferences_GetAfterLogin(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-pref-1", Email: "pref@example.com", Name: "Pref User",
		},
	}
	app, queries, authCtrl := setupE2EApp(t, oauth)

	user, _, accessToken := loginViaCallback(t, app, queries)
	_ = user
	_ = authCtrl

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, readJSON(resp, &body))
	assert.Equal(t, "pt", body["language_to_translate"])
	assert.Equal(t, "mixed", body["ai_personality"])
}

func TestE2E_Preferences_UpdateAndVerifyNewToken(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-pref-2", Email: "pref2@example.com", Name: "Pref User 2",
		},
	}
	app, queries, _ := setupE2EApp(t, oauth)

	_, _, accessToken := loginViaCallback(t, app, queries)

	body := `{"language_to_translate":"en","ai_personality":"informative"}`
	req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, readJSON(resp, &result))

	newToken, ok := result["access_token"].(string)
	require.True(t, ok && newToken != "", "new access_token should be returned")
	assert.NotEqual(t, accessToken, newToken, "new token should differ from old token")

	prefs, ok := result["preferences"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "en", prefs["language_to_translate"])
	assert.Equal(t, "informative", prefs["ai_personality"])

	// New token should carry the updated preferences.
	getReq, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)
	getReq.Header.Set("Authorization", "Bearer "+newToken)

	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var updatedBody map[string]interface{}
	require.NoError(t, readJSON(getResp, &updatedBody))
	assert.Equal(t, "en", updatedBody["language_to_translate"])
}

func TestE2E_Preferences_InvalidValues(t *testing.T) {
	requireNotProduction(t)

	oauth := external.MockGoogleOAuth{
		UserInfo: external.GoogleUserInfo{
			ID: "e2e-pref-3", Email: "pref3@example.com", Name: "Pref User 3",
		},
	}
	app, queries, _ := setupE2EApp(t, oauth)
	_, _, accessToken := loginViaCallback(t, app, queries)

	cases := []struct {
		name string
		body string
	}{
		{"invalid language_to_translate", `{"language_to_translate":"elvish","ai_personality":"mixed"}`},
		{"invalid ai_personality", `{"language_to_translate":"pt","ai_personality":"chaotic"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPut, "/v1/users/me/preferences", strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+accessToken)

			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestE2E_Preferences_Unauthenticated(t *testing.T) {
	requireNotProduction(t)

	app, _, _ := setupE2EApp(t, external.MockGoogleOAuth{})

	req, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
