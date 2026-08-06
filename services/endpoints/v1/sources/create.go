package sources

import (
	"errors"
	"net/url"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func CreateSource(ctrl controllers.SourceControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		var req schemas.CreateSourceRequest
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
			source, err = ctrl.Create(c.Context(), q, req.Name, req.URL, req.URLRss)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrSourceAlreadyExists) {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "source with this URL already exists"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create source"})
		}

		return c.Status(fiber.StatusCreated).JSON(toSourceResponse(source))
	}
}

func isValidURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func toSourceResponse(s db.Source) schemas.SourceResponse {
	resp := schemas.SourceResponse{
		ID:         s.ID,
		Status:     s.Status,
		Name:       s.Name,
		URL:        s.Url,
		URLRss:     s.UrlRss,
		CreatedAt:  s.CreatedAt.Time,
		ModifiedAt: s.ModifiedAt.Ptr(),
	}
	return resp
}
