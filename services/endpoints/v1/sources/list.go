package sources

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func ListSources(ctrl controllers.SourceControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		urlFilter := strings.ToLower(c.Query("url"))
		statusFilter := c.Query("status")

		var sources []db.Source
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			sources, err = ctrl.List(c.Context(), q)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve sources"})
		}

		filtered := applyFilters(sources, urlFilter, statusFilter)

		result := make([]schemas.SourceResponse, len(filtered))
		for i, s := range filtered {
			result[i] = toSourceResponse(s)
		}

		return c.JSON(result)
	}
}

func applyFilters(sources []db.Source, urlFilter, statusFilter string) []db.Source {
	if urlFilter == "" && statusFilter == "" {
		return sources
	}

	var out []db.Source
	for _, s := range sources {
		if urlFilter != "" && !strings.Contains(strings.ToLower(s.Url), urlFilter) {
			continue
		}
		if statusFilter != "" {
			wantActive := statusFilter == "true"
			isActive := s.Status == 1
			if wantActive != isActive {
				continue
			}
		}
		out = append(out, s)
	}

	if out == nil {
		return []db.Source{}
	}
	return out
}
