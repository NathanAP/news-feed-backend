package feeds

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func CreateFeed(ctrl controllers.FeedControllerInterface, runTx controllers.TransactionRunner) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		claims := middlewares.GetClaims(c)

		var req schemas.CreateFeedRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		if msg := validateFeedInput(req.Name, req.Keywords); msg != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}

		var feed db.Feed
		err := runTx(c.Context(), func(q db.Querier) error {
			var err error
			feed, err = ctrl.Create(c.Context(), q, claims.UserID, req.Name, req.Keywords)
			return err
		})
		if err != nil {
			if errors.Is(err, controllers.ErrFeedLimitReached) {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "feed limit reached (maximum 5 active feeds per user)"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create feed"})
		}

		response, err := toFeedResponse(feed)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build feed response"})
		}

		return c.Status(fiber.StatusCreated).JSON(response)
	}
}

// validateFeedInput enforces the structural rules for a feed payload. Returns an empty
// string when valid, or the error message otherwise.
func validateFeedInput(name string, keywords []string) string {
	if strings.TrimSpace(name) == "" {
		return "name is required"
	}
	if len(name) > schemas.FeedNameMaxLength {
		return "name must have at most 120 characters"
	}
	if len(keywords) < schemas.FeedKeywordsMin {
		return "keywords must have at least 5 items"
	}
	if len(keywords) > schemas.FeedKeywordsMax {
		return "keywords must have at most 20 items"
	}
	for _, k := range keywords {
		if strings.TrimSpace(k) == "" {
			return "keywords must not contain empty values"
		}
	}
	return ""
}

func toFeedResponse(f db.Feed) (schemas.FeedResponse, error) {
	keywords, err := controllers.DecodeKeywords(f.Keywords)
	if err != nil {
		return schemas.FeedResponse{}, err
	}

	resp := schemas.FeedResponse{
		ID:        f.ID,
		Status:    f.Status == 1,
		Name:      f.Name,
		Keywords:  keywords,
		UserID:    f.UserID,
		CreatedAt: f.CreatedAt,
	}
	if f.ModifiedAt.Valid {
		t := f.ModifiedAt.Time
		resp.ModifiedAt = &t
	}
	return resp, nil
}
