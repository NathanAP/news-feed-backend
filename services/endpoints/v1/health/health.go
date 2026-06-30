package health

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// Check reports liveness plus a small control snapshot: the project version, the global
// app_status (so a client can detect maintenance with a single cheap call) and the current
// server time in UTC. It is exempt from the app_status guard so monitoring and clients keep
// working while the application is under maintenance.
func Check(systemCtrl controllers.SystemControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		appStatus := true
		if err := runTx(c.Context(), func(q db.Querier) error {
			system, err := systemCtrl.Get(c.Context(), q)
			if err != nil {
				return err
			}
			appStatus = system.AppStatus == 1
			return nil
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		return c.JSON(fiber.Map{
			"status":      "ok",
			"version":     os.Getenv("PROJECT_VERSION"),
			"app_status":  appStatus,
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
