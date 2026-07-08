package feeds

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/pagination"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// FeedArticles serves GET /v1/feeds/{id}/articles: the articles that landed in one of the requesting
// user's feeds, each with its is_read state for that feed. This is the product's central read path.
// The feed must belong to the caller — another user's (or a missing) feed returns 404, never leaking
// its existence; a feed the user owns but with no matching articles returns 200 with an empty list.
// Optional query filters: is_read (true|false) and a created_at window (period_starting_at /
// period_ending_at, RFC3339 UTC, both bounds inclusive and independent). with_sources=true also
// populates each article's source (conventions.md: any value other than "true" is ignored, never an
// error). The response follows the standard paginated envelope ({ docs, pagination }).
func FeedArticles(feedCtrl controllers.FeedControllerInterface, afCtrl controllers.ArticleFeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
		}

		isReadFilter, err := parseIsRead(c.Query("is_read"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		startAt, err := parseUTCTime(c.Query("period_starting_at"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "period_starting_at must be an RFC3339 UTC date"})
		}
		endAt, err := parseUTCTime(c.Query("period_ending_at"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "period_ending_at must be an RFC3339 UTC date"})
		}
		withSources := c.Query("with_sources") == "true"

		var rows []db.ListArticlesByFeedForUserRow
		err = runTx(c.Context(), func(q db.Querier) error {
			// Ownership check first so we can tell "not your feed / missing" (404) apart from an
			// owned-but-empty feed (200). Both would otherwise produce zero rows below.
			if _, e := feedCtrl.FindByID(c.Context(), q, id, claims.UserID); e != nil {
				return e
			}
			var e error
			rows, e = afCtrl.ListArticlesByFeedForUser(c.Context(), q, id, claims.UserID)
			return e
		})
		if err != nil {
			if errors.Is(err, controllers.ErrFeedNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "feed not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve feed articles"})
		}

		result := make([]schemas.ArticleResponse, 0, len(rows))
		for _, row := range rows {
			if isReadFilter != nil && (row.IsRead == 1) != *isReadFilter {
				continue
			}
			if startAt != nil && row.CreatedAt.Before(*startAt) {
				continue
			}
			if endAt != nil && row.CreatedAt.After(*endAt) {
				continue
			}

			response, err := rowToArticleResponse(row, withSources)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build article response"})
			}
			result = append(result, response)
		}

		params := pagination.ParseParams(c.Query("page"), c.Query("page_size"))
		return c.JSON(pagination.Paginate(result, params))
	}
}

// parseIsRead reads the optional is_read filter. Empty means "no filter" (nil); "true"/"false" set
// it; anything else is a bad request.
func parseIsRead(raw string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	switch raw {
	case "true":
		v := true
		return &v, nil
	case "false":
		v := false
		return &v, nil
	default:
		return nil, errors.New("is_read must be true or false")
	}
}

// parseUTCTime reads an optional RFC3339 date bound and normalizes it to UTC (dates are UTC across
// the application). Empty means "no bound" (nil); an unparseable value is an error.
func parseUTCTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}

// rowToArticleResponse maps a feed-article row to the shared ArticleResponse. is_read is always
// resolved here (the article is in this feed, so the state is definite — never nil, unlike the
// single-article endpoint where the article may be in none of the user's feeds). Source is mapped
// only when withSources is true, even though the row always carries it (the query always joins
// sources, per conventions.md's with_{related_table_name} pattern): the join is cheap, so the same
// base query serves both cases, and only the response shaping differs.
func rowToArticleResponse(row db.ListArticlesByFeedForUserRow, withSources bool) (schemas.ArticleResponse, error) {
	keywords, err := controllers.DecodeKeywords(row.Keywords)
	if err != nil {
		return schemas.ArticleResponse{}, err
	}

	resp := schemas.ArticleResponse{
		ID:          row.ID,
		Status:      row.Status == 1,
		Title:       row.Title,
		Content:     row.Content,
		URLOriginal: row.UrlOriginal,
		Keywords:    keywords,
		SourceID:    row.SourceID,
		CreatedAt:   row.CreatedAt,
	}
	if row.LanguageOriginal.Valid {
		l := row.LanguageOriginal.String
		resp.LanguageOriginal = &l
	}
	if row.ModifiedAt.Valid {
		t := row.ModifiedAt.Time
		resp.ModifiedAt = &t
	}
	isRead := row.IsRead == 1
	resp.IsRead = &isRead

	if withSources {
		source := schemas.SourceResponse{
			ID:        row.SourceID,
			Status:    row.SourceStatus == 1,
			Name:      row.SourceName,
			URL:       row.SourceUrl,
			URLRss:    row.SourceUrlRss,
			CreatedAt: row.SourceCreatedAt,
		}
		if row.SourceModifiedAt.Valid {
			t := row.SourceModifiedAt.Time
			source.ModifiedAt = &t
		}
		resp.Source = &source
	}
	return resp, nil
}
