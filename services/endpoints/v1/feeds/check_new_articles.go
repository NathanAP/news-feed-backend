package feeds

import (
	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// CheckForNewArticles serves GET /v1/feeds/check-for-new-articles: a lightweight poll that tells the
// client which of the caller's active feeds currently have unread articles, and how many. It is a
// status indicator (not a record listing), so it is not paginated — a user has at most a handful of
// feeds. The response is a flat object keyed by feed id with the unread count as value:
//
//	{ "<feed_id>": 3, "<feed_id>": 1 }
//
// Only feeds with at least one unread article appear; when the user has none, the response is an
// empty object ({}). Everything is scoped to the requesting user and computed in SQL. This is the
// endpoint a future SSE/WebSocket would replace for push delivery.
func CheckForNewArticles(afCtrl controllers.ArticleFeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		var rows []db.CountUnreadArticlesByFeedForUserRow
		err := runTx(c.Context(), func(q db.Querier) error {
			var e error
			rows, e = afCtrl.CountUnreadByFeedForUser(c.Context(), q, claims.UserID)
			return e
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to check for new articles"})
		}

		result := make(fiber.Map, len(rows))
		for _, row := range rows {
			result[row.FeedID] = row.UnreadCount
		}
		return c.JSON(result)
	}
}
