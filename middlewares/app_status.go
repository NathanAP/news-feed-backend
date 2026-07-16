package middlewares

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// NewAppStatusMiddleware guards business routes against the global maintenance switch. While
// app_status is off (0) every request that reaches this middleware gets 503. The flag is read
// from the database on each request (the source of truth), so toggling it through the
// control-panel endpoint takes effect on the very next request — no in-memory cache, no
// invalidation step. Routes that must survive maintenance (the health check and the toggle
// endpoint itself) are registered before this middleware is mounted and are never reached by it.
func NewAppStatusMiddleware(systemCtrl controllers.SystemControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
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

		if !active {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service unavailable: the application is under maintenance",
			})
		}

		return c.Next()
	}
}
