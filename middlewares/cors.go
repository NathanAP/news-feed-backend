package middlewares

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// NewCORSMiddleware builds the CORS handler from CORS_ALLOWED_ORIGINS (a comma-separated list of
// allowed origins, each a full scheme+host+port, e.g. http://localhost:5173). The web client runs
// on a different origin than the API, so without these headers the browser blocks every
// cross-origin request. Credentials are intentionally not enabled: auth travels as a Bearer token,
// not a cookie, so there is no cookie to allow and the simpler non-credentialed CORS is enough.
//
// When CORS_ALLOWED_ORIGINS is empty it logs a warning and returns a no-op passthrough — no CORS
// headers are emitted, so browsers fail closed (no cross-origin client is allowed). This avoids
// Fiber's default of falling back to "*" on an empty AllowOrigins, which would silently open the
// API to every origin. The API still boots and keeps serving same-origin / non-browser clients.
func NewCORSMiddleware() fiber.Handler {
	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if origins == "" {
		log.Println("CORS_ALLOWED_ORIGINS is empty: CORS disabled, no cross-origin browser client will be allowed")
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	return cors.New(cors.Config{
		AllowOrigins: origins,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Authorization,Content-Type",
	})
}
