package articles

import (
	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// ListArticles serves GET /v1/articles: every article registered in the application (articles are
// global — any user may read any of them; the per-feed read path is GET /v1/feeds/{id}/articles).
// Optional query filter: url, a case-insensitive substring match against url_original. No status
// filter is exposed by convention — soft-deleted articles must never appear in a list, so the query
// hardcodes the active predicate. Filtering and pagination happen in SQL; the response follows the
// standard paginated envelope.
func ListArticles(ctrl controllers.ArticleControllerInterface, outboundCtrl controllers.ArticleOutboundLinkControllerInterface, runTx controllers.TransactionRunner, clientURL string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		filter := controllers.ListArticlesFilter{
			URL:  c.Query("url"),
			Page: pagination.ParseParams(c.Query("page"), c.Query("page_size")),
		}

		var articles []db.Article
		var total int64
		var links []db.ArticleOutboundLink
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			articles, total, err = ctrl.List(c.Context(), q, filter)
			if err != nil {
				return err
			}
			// One batch fetch of the whole page's outbound links for the read-time swap (no N+1);
			// ids are globally unique so a single map resolves every article on the page.
			links, err = outboundCtrl.ListByArticleIDs(c.Context(), q, articleIDs(articles))
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve articles"})
		}

		byID := controllers.OutboundLinksByID(links)
		docs := make([]schemas.ArticleResponse, 0, len(articles))
		for _, a := range articles {
			response, err := toArticleResponse(a)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build article response"})
			}
			response.Content = outboundlinks.Resolve(response.Content, byID, clientURL)
			docs = append(docs, response)
		}

		return c.JSON(pagination.BuildResponse(docs, total, filter.Page))
	}
}

// articleIDs collects the ids of a page of articles, for the batch outbound-link lookup.
func articleIDs(articles []db.Article) []string {
	ids := make([]string, len(articles))
	for i, a := range articles {
		ids[i] = a.ID
	}
	return ids
}
