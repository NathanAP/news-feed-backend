package middlewares

import (
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// defaultAllowedHeaders is what the API needs on its own: the Bearer token and the body's content
// type. Anything beyond this is environment-specific and comes from CORS_ALLOWED_HEADERS.
const defaultAllowedHeaders = "Authorization,Content-Type"

// NewCORSMiddleware builds the CORS handler from CORS_ALLOWED_ORIGINS (a comma-separated list of
// allowed origins, each a full scheme+host+port, e.g. http://localhost:5173). The web client runs
// on a different origin than the API, so without these headers the browser blocks every
// cross-origin request. Credentials are intentionally not enabled: auth travels as a Bearer token,
// not a cookie, so there is no cookie to allow and the simpler non-credentialed CORS is enough.
//
// The request headers the client may send come from CORS_ALLOWED_HEADERS, falling back to
// defaultAllowedHeaders. It is configurable for the same reason the origins are: which headers a
// client legitimately sends depends on how it reaches the API, not on the API. Concretely, a client
// talking through a tunnel may have to send a tunnel-specific header (ngrok's free tier serves an
// interstitial HTML page to anything with a browser User-Agent unless the request carries
// `ngrok-skip-browser-warning`), and a header the preflight does not allow is a header the browser
// refuses to send.
//
// When CORS_ALLOWED_ORIGINS is empty it logs a warning and returns a no-op passthrough — no CORS
// headers are emitted, so browsers fail closed (no cross-origin client is allowed). This avoids
// Fiber's default of falling back to "*" on an empty AllowOrigins, which would silently open the
// API to every origin. The API still boots and keeps serving same-origin / non-browser clients.
func NewCORSMiddleware() fiber.Handler {
	// Fiber v3 takes []string where v2 took a comma-separated string and split it internally, so the
	// splitting moved here. Guarding on the PARSED list rather than on the raw variable also closes a
	// gap the v2 version had: a value of "," or "  " is non-empty as a string but allows no origin,
	// and handing that to the middleware is exactly the case that could fall back to "*".
	origins := splitAndTrim(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(origins) == 0 {
		log.Println("CORS_ALLOWED_ORIGINS is empty: CORS disabled, no cross-origin browser client will be allowed")
		return func(c fiber.Ctx) error { return c.Next() }
	}

	// Empty (or unset) falls back rather than allowing nothing: an operator who does not care about
	// extra headers should not have to know this variable exists to have a working client.
	headers := splitAndTrim(os.Getenv("CORS_ALLOWED_HEADERS"))
	if len(headers) == 0 {
		headers = splitAndTrim(defaultAllowedHeaders)
	}

	return cors.New(cors.Config{
		AllowOrigins: origins,
		AllowMethods: []string{
			fiber.MethodGet, fiber.MethodPost, fiber.MethodPut,
			fiber.MethodDelete, fiber.MethodOptions,
		},
		AllowHeaders: headers,
	})
}

// splitAndTrim turns a comma-separated environment value into the slice the middleware wants,
// dropping blanks. Blank-dropping is what makes the length check above meaningful: without it,
// "a,,b" would carry an empty origin into the allow-list.
func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
