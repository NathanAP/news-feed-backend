package sources

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// ListSources serves GET /v1/sources: the sources every user can see (they are public, and
// administrator-managed). Optional query filters: url and name, each a case-insensitive substring
// match, applied together when both are given. No status filter is exposed by convention —
// soft-deleted sources must never appear in a list, so the query hardcodes the active predicate.
// Filtering and pagination happen in SQL; the response follows the standard paginated envelope.
func ListSources(ctrl controllers.SourceControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		filter := controllers.ListSourcesFilter{
			URL:  c.Query("url"),
			Name: c.Query("name"),
			Page: pagination.ParseParams(c.Query("page"), c.Query("page_size")),
		}

		var sources []db.Source
		var total int64
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			sources, total, err = ctrl.List(c.Context(), q, filter)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve sources"})
		}

		docs := make([]schemas.SourceResponse, len(sources))
		for i, s := range sources {
			docs[i] = toSourceResponse(s)
		}

		return c.JSON(pagination.BuildResponse(docs, total, filter.Page))
	}
}
