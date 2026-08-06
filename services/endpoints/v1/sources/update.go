package sources

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func UpdateSource(ctrl controllers.SourceControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		var req schemas.UpdateSourceRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		if req.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
		}
		if len(req.Name) > schemas.SourceNameMaxLength {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name must be at most 120 characters"})
		}
		if req.URL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url is required"})
		}
		if req.URLRss == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url_rss is required"})
		}
		if !isValidURL(req.URL) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url must be a valid http or https URL"})
		}
		if !isValidURL(req.URLRss) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url_rss must be a valid http or https URL"})
		}

		var source db.Source
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			source, err = ctrl.Update(c.Context(), q, id, req.Name, req.URL, req.URLRss)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrSourceNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "source not found"})
			}
			if errors.Is(err, controllers.ErrSourceAlreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "source with this URL already exists"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update source"})
		}

		return c.JSON(toSourceResponse(source))
	}
}
