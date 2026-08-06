package system_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func putAppStatus(t *testing.T, app *fiber.App, body string) *http.Response {
	t.Helper()
	return putAppStatusWithHeader(t, app, body, authHeader(t))
}

func putAppStatusWithHeader(t *testing.T, app *fiber.App, body, header string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, "/v1/system/app-status", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

// ── Administrator guard ────────────────────────────────────────────────────────

// The toggle was reachable by anyone until 0.40 — an unauthenticated caller could take the whole API
// down, and the guard exempts it from maintenance precisely so it is always reachable. These three
// tests are what keep that exemption from being an open door.

func TestUpdateAppStatus_RejectsAnonymous(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatusWithHeader(t, defaultApp(), `{"app_status":false}`, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUpdateAppStatus_RejectsRegularUser(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatus(t, buildAppAsRegularUser(&mockSystemCtrl{}), `{"app_status":false}`)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// The guard runs before the body is parsed, so a regular user gets 403 even with a payload the
// handler would have rejected. Authorization is not supposed to depend on the request being valid.
func TestUpdateAppStatus_RejectsRegularUserBeforeValidation(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatus(t, buildAppAsRegularUser(&mockSystemCtrl{}), `{}`)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateAppStatus_EnableSuccess(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatus(t, defaultApp(), `{"app_status":true}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, true, result["app_status"])
}

func TestUpdateAppStatus_DisableSuccess(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatus(t, defaultApp(), `{"app_status":false}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, readJSON(resp, &result))
	assert.Equal(t, false, result["app_status"])
}

func TestUpdateAppStatus_MissingField(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatus(t, defaultApp(), `{}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateAppStatus_InvalidBody(t *testing.T) {
	requireNotProduction(t)

	resp := putAppStatus(t, defaultApp(), "not json")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateAppStatus_DBError(t *testing.T) {
	requireNotProduction(t)

	ctrl := &mockSystemCtrl{
		updateAppStatusFn: func(_ context.Context, _ db.Querier, _ bool) (db.System, error) {
			return db.System{}, assert.AnError
		},
	}
	resp := putAppStatus(t, buildApp(ctrl), `{"app_status":true}`)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
