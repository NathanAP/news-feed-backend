package auth_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidate_Success(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate", []byte(`{"refresh_token_id":"some-token-id"}`))

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestInvalidate_MissingBody(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestInvalidate_MissingRefreshTokenID(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate", []byte(`{}`))

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── Administrator guard (0.40) ─────────────────────────────────────────────────

// This route revokes someone else's session by id. Until 0.40 it required no authentication at all,
// so anyone able to reach the API could sign an arbitrary user out.

func TestInvalidate_RejectsAnonymous(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestInvalidate_RejectsRegularUser(t *testing.T) {
	requireNotProduction(t)

	app := setupAppAsRegularUser(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate", []byte(`{"refresh_token_id":"some-token-id"}`))

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
