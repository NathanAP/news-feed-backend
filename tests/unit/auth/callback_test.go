package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/oauthstate"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

func validState(t *testing.T) string {
	t.Helper()
	state, err := oauthstate.Generate([]byte(jwtmock.TestJWTSecret), testutils.TestOAuthRedirectURI)
	require.NoError(t, err)
	return state
}

func TestGoogleCallback_Success_RedirectsWithTokensInFragment(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		handleGoogleCallbackFn: func(ctx context.Context, code string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
			}, nil
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code&state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	access, refresh := testutils.ParseTokensFromRedirect(t, resp.Header.Get("Location"))
	assert.Equal(t, "test-access-token", access)
	assert.Equal(t, "test-refresh-token", refresh)
}

func TestGoogleCallback_MissingState_401(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGoogleCallback_MissingCode_401(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGoogleCallback_AuthFailure_401(t *testing.T) {
	requireNotProduction(t)

	authCtrl := &mockAuthCtrl{
		handleGoogleCallbackFn: func(ctx context.Context, code string) (schemas.AuthResponse, error) {
			return schemas.AuthResponse{}, errors.New("oauth exchange failed")
		},
	}

	app := setupApp(authCtrl, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=bad-code&state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// User cancellation at Google: the callback bounces back to the client redirect_uri with ?error=.
func TestGoogleCallback_UserCancelled_RedirectsWithError(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodGet, "/v1/auth/google/callback?error=access_denied&state="+url.QueryEscape(validState(t)), nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	location := resp.Header.Get("Location")
	assert.Contains(t, location, testutils.TestOAuthRedirectURI)
	assert.Contains(t, location, "error=access_denied")
}
