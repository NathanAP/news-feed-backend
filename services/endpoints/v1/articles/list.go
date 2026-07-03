package articles

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func ListArticles(ctrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		urlFilter := strings.ToLower(c.Query("url"))

		var articles []db.Article
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			articles, err = ctrl.List(c.Context(), q)
			return err
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve articles"})
		}

		filtered := applyFilters(articles, urlFilter)

		result := make([]schemas.ArticleResponse, 0, len(filtered))
		for _, a := range filtered {
			response, err := toArticleResponse(a)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build article response"})
			}
			result = append(result, response)
		}

		params := pagination.ParseParams(c.Query("page"), c.Query("page_size"))
		return c.JSON(pagination.Paginate(result, params))
	}
}

// applyFilters narrows the active articles by url_original substring. Inactive records never
// reach here (the query already filters status = 1 AND removed_at IS NULL).
func applyFilters(articles []db.Article, urlFilter string) []db.Article {
	if urlFilter == "" {
		return articles
	}

	var out []db.Article
	for _, a := range articles {
		if !strings.Contains(strings.ToLower(a.UrlOriginal), urlFilter) {
			continue
		}
		out = append(out, a)
	}
	return out
}
