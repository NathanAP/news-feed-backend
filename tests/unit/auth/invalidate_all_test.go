package auth_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidateAll_Success(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate-all?user_id=some-user-id", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestInvalidateAll_MissingUserID(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate-all", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── Administrator guard (0.40) ─────────────────────────────────────────────────

// This one signs out *every* session of a chosen user, and it too was reachable with no credentials
// at all before 0.40.

func TestInvalidateAll_RejectsAnonymous(t *testing.T) {
	requireNotProduction(t)

	app := setupApp(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req, err := http.NewRequest(http.MethodDelete, "/v1/auth/invalidate-all?user_id=some-user-id", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestInvalidateAll_RejectsRegularUser(t *testing.T) {
	requireNotProduction(t)

	app := setupAppAsRegularUser(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate-all?user_id=some-user-id", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// A regular user must not reach this route even when targeting their own id — /auth/logout is the
// route for ending your own session, and passing your own id is the obvious way someone would try.
func TestInvalidateAll_RejectsRegularUserTargetingThemselves(t *testing.T) {
	requireNotProduction(t)

	app := setupAppAsRegularUser(&mockAuthCtrl{}, validRefreshTokenCtrl())
	req := buildAuthRequest(t, http.MethodDelete, "/v1/auth/invalidate-all?user_id=01900000-0000-7000-8000-000000000001", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
