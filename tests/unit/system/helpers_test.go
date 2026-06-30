package system_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	systemendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/system"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func fakeTxRunner(_ context.Context, fn func(q db.Querier) error) error {
	return fn(nil)
}

func readJSON(resp *http.Response, target any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

const testSystemID = "01900000-0000-7000-8000-000000000001"

// mockSystemCtrl holds optional function overrides for each method.
type mockSystemCtrl struct {
	getFn             func(ctx context.Context, q db.Querier) (db.System, error)
	updateAppStatusFn func(ctx context.Context, q db.Querier, active bool) (db.System, error)
}

func (m *mockSystemCtrl) Get(ctx context.Context, q db.Querier) (db.System, error) {
	if m.getFn != nil {
		return m.getFn(ctx, q)
	}
	return db.System{ID: testSystemID, AppStatus: 1}, nil
}

func (m *mockSystemCtrl) UpdateAppStatus(ctx context.Context, q db.Querier, active bool) (db.System, error) {
	if m.updateAppStatusFn != nil {
		return m.updateAppStatusFn(ctx, q, active)
	}
	status := int64(0)
	if active {
		status = 1
	}
	return db.System{ID: testSystemID, AppStatus: status}, nil
}

var _ controllers.SystemControllerInterface = (*mockSystemCtrl)(nil)

func buildApp(ctrl controllers.SystemControllerInterface) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Put("/v1/system/app-status", systemendpoints.UpdateAppStatus(ctrl, fakeTxRunner))
	return app
}

func defaultApp() *fiber.App {
	return buildApp(&mockSystemCtrl{})
}
