package system

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// UpdateAppStatus toggles the global maintenance switch (app_status). It is open for now
// (admin-future) and is intentionally exempt from the app_status guard, so the application can
// always be brought back online through the API instead of editing the database directly.
func UpdateAppStatus(ctrl controllers.SystemControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.UpdateAppStatusRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.AppStatus == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "app_status is required"})
		}

		var system db.System
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			system, err = ctrl.UpdateAppStatus(c.Context(), q, *req.AppStatus)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update app status"})
		}

		return c.JSON(toSystemResponse(system))
	}
}

func toSystemResponse(s db.System) schemas.SystemResponse {
	resp := schemas.SystemResponse{
		ID:        s.ID,
		AppStatus: s.AppStatus,
		CreatedAt: s.CreatedAt,
	}
	if s.LastArticleDiscoveryAt.Valid {
		t := s.LastArticleDiscoveryAt.Time
		resp.LastArticleDiscoveryAt = &t
	}
	if s.ModifiedAt.Valid {
		t := s.ModifiedAt.Time
		resp.ModifiedAt = &t
	}
	return resp
}
