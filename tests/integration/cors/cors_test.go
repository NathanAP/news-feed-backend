package cors_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/middlewares"
)

// buildApp mounts the real CORS middleware (which reads CORS_ALLOWED_ORIGINS at construction) on a
// minimal app with a single route, so the tests exercise exactly what main.go wires in production.
func buildApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(middlewares.NewCORSMiddleware())
	app.Get("/ping", func(c fiber.Ctx) error {
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

// preflight asks permission for the given request headers and returns the response, so the
// CORS_ALLOWED_HEADERS tests below read like the browser exchange they stand for.
func preflight(t *testing.T, app *fiber.App, requestHeaders string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodOptions, "/ping", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", requestHeaders)

	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

// Unset CORS_ALLOWED_HEADERS falls back to the API's own needs, so a deployment that never heard of
// this variable still has a working client.
func TestCORS_AllowedHeadersDefaultsWhenUnset(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("CORS_ALLOWED_HEADERS", "")
	app := buildApp(t)

	allowed := preflight(t, app, "Authorization").Header.Get("Access-Control-Allow-Headers")
	require.Contains(t, allowed, "Authorization")
	require.Contains(t, allowed, "Content-Type")
}

// The reason the variable exists: a client reaching the API through a tunnel has to send a
// tunnel-specific header, and the browser only sends what the preflight allows.
func TestCORS_AllowedHeadersHonoursEnv(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("CORS_ALLOWED_HEADERS", "Authorization,Content-Type,ngrok-skip-browser-warning")
	app := buildApp(t)

	allowed := preflight(t, app, "Authorization,ngrok-skip-browser-warning").Header.Get("Access-Control-Allow-Headers")
	require.Contains(t, allowed, "ngrok-skip-browser-warning")
	require.Contains(t, allowed, "Authorization")
}

// A header outside the configured list must not be advertised: that is what makes the browser
// refuse to send it, which is exactly the failure this variable exists to let an operator resolve.
func TestCORS_AllowedHeadersExcludesUnlisted(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("CORS_ALLOWED_HEADERS", "Authorization,Content-Type")
	app := buildApp(t)

	allowed := preflight(t, app, "X-Made-Up").Header.Get("Access-Control-Allow-Headers")
	require.NotContains(t, allowed, "X-Made-Up")
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
