package feeds

import (
	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// ListFeeds serves GET /v1/feeds: the requesting user's own feeds — a feed is only ever visible to
// the user who created it, so the scope comes from the token, never from the query. Optional query
// filter: name, a case-insensitive substring match. No status filter is exposed by convention —
// soft-deleted feeds must never appear in a list, so the query hardcodes the active predicate.
// Filtering and pagination happen in SQL; the response follows the standard paginated envelope.
func ListFeeds(ctrl controllers.FeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)
		filter := controllers.ListFeedsFilter{
			Name: c.Query("name"),
			Page: pagination.ParseParams(c.Query("page"), c.Query("page_size")),
		}

		var feeds []db.Feed
		var total int64
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			feeds, total, err = ctrl.List(c.Context(), q, claims.UserID, filter)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve feeds"})
		}

		docs := make([]schemas.FeedResponse, 0, len(feeds))
		for _, f := range feeds {
			response, err := toFeedResponse(f)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build feed response"})
			}
			docs = append(docs, response)
		}

		return c.JSON(pagination.BuildResponse(docs, total, filter.Page))
	}
}
