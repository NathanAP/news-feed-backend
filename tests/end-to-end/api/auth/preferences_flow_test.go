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
	assert.Equal(t, "dark", body["theme"])
	assert.Equal(t, "pt", body["language"])
	assert.Equal(t, true, body["translate_content"])
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

	body := `{"theme":"light","language":"en","translate_content":false,"ai_personality":"informative"}`
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
	assert.Equal(t, "light", prefs["theme"])
	assert.Equal(t, "en", prefs["language"])
	assert.Equal(t, false, prefs["translate_content"])
	assert.Equal(t, "informative", prefs["ai_personality"])

	// New token should contain updated preferences
	getReq, err := http.NewRequest(http.MethodGet, "/v1/users/me/preferences", nil)
	require.NoError(t, err)
	getReq.Header.Set("Authorization", "Bearer "+newToken)

	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var updatedBody map[string]interface{}
	require.NoError(t, readJSON(getResp, &updatedBody))
	assert.Equal(t, "light", updatedBody["theme"])
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
		{"invalid theme", `{"theme":"rainbow","language":"pt","translate_content":true,"ai_personality":"mixed"}`},
		{"invalid language", `{"theme":"dark","language":"elvish","translate_content":true,"ai_personality":"mixed"}`},
		{"invalid ai_personality", `{"theme":"dark","language":"pt","translate_content":true,"ai_personality":"chaotic"}`},
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
