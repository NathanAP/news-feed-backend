package feeds

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func ListFeeds(ctrl controllers.FeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)
		nameFilter := strings.ToLower(c.Query("name"))

		var feeds []db.Feed
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			feeds, err = ctrl.List(c.Context(), q, claims.UserID)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve feeds"})
		}

		filtered := applyFilters(feeds, nameFilter)

		result := make([]schemas.FeedResponse, 0, len(filtered))
		for _, f := range filtered {
			response, err := toFeedResponse(f)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build feed response"})
			}
			result = append(result, response)
		}

		params := pagination.ParseParams(c.Query("page"), c.Query("page_size"))
		return c.JSON(pagination.Paginate(result, params))
	}
}

// applyFilters narrows the user's feeds by name substring. Inactive and other users' feeds
// never reach here (the query already filters user_id, status = 1 AND removed_at IS NULL).
func applyFilters(feeds []db.Feed, nameFilter string) []db.Feed {
	if nameFilter == "" {
		return feeds
	}

	var out []db.Feed
	for _, f := range feeds {
		if !strings.Contains(strings.ToLower(f.Name), nameFilter) {
			continue
		}
		out = append(out, f)
	}
	return out
}
