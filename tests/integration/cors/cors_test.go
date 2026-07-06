package cors_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
)

// buildApp mounts the real CORS middleware (which reads CORS_ALLOWED_ORIGINS at construction) on a
// minimal app with a single route, so the tests exercise exactly what main.go wires in production.
func buildApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middlewares.NewCORSMiddleware())
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})
	return app
}

func TestCORS_AllowedOriginGetsHeader(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	app := buildApp(t)

	req, err := http.NewRequest(http.MethodGet, "/ping", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:5173")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "http://localhost:5173", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightReturns204WithAllowHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	app := buildApp(t)

	req, err := http.NewRequest(http.MethodOptions, "/ping", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "http://localhost:5173", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
}

// A cross-origin request from an origin not in the allowlist must not receive the
// Access-Control-Allow-Origin header, so the browser blocks it.
func TestCORS_DisallowedOriginGetsNoHeader(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	app := buildApp(t)

	req, err := http.NewRequest(http.MethodGet, "/ping", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://evil.example.com")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

// With CORS_ALLOWED_ORIGINS unset the middleware fails closed: no CORS header is emitted (it must
// NOT fall back to "*"), so no cross-origin browser client is allowed.
func TestCORS_EmptyEnvFailsClosed(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	app := buildApp(t)

	req, err := http.NewRequest(http.MethodGet, "/ping", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:5173")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}
