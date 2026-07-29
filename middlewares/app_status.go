package middlewares

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// NewAppStatusMiddleware guards business routes against the global maintenance switch. While
// app_status is off every request that reaches this middleware gets 503, unless it comes from an
// administrator — PROJECT.md: an administrator is never blocked by the control panel's own switches.
// The flag is read from the database on each request (the source of truth), so toggling it through
// the control-panel endpoint takes effect on the very next request — no in-memory cache, no
// invalidation step. Routes that must survive maintenance (the health check, the toggle endpoint and
// the whole /auth group) are registered before this middleware is mounted and are never reached by it.
//
// The administrator check costs nothing on the normal path: it only runs once the switch is already
// off, which is the rare state. So a healthy application still pays exactly one query per request
// here, the same as before this exemption existed.
func NewAppStatusMiddleware(systemCtrl controllers.SystemControllerInterface, runTx controllers.TransactionRunner, adminResolver *AdminResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var active bool
		err := runTx(c.Context(), func(q db.Querier) error {
			system, err := systemCtrl.Get(c.Context(), q)
			if err != nil {
				return err
			}
			active = system.AppStatus
			return nil
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		if active {
			return c.Next()
		}

		// Identity has to be resolved from the request itself: this middleware is global and runs
		// ahead of the per-route auth middleware, so nothing has populated the claims yet. The
		// resolver is fail-closed — anything it cannot verify is treated as a regular user.
		if adminResolver.IsRequestFromAdmin(c) {
			return c.Next()
		}

		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "service unavailable: the application is under maintenance",
		})
	}
}
