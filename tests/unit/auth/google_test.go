package auth_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func TestGoogleLogin_ValidRedirectURI_Redirects(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google?redirect_uri="+url.QueryEscape(testutils.TestOAuthRedirectURI), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	location := resp.Header.Get("Location")
	require.NotEmpty(t, location)
	// The state carrying the redirect_uri must be present in the Google auth URL.
	assert.Contains(t, location, "state=")
}

func TestGoogleLogin_MissingRedirectURI_400(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGoogleLogin_RedirectURINotAllowlisted_400(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google?redirect_uri="+url.QueryEscape("https://evil.example.com/steal"), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
