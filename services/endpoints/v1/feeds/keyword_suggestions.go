package feeds

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// SuggestKeywords serves GET /v1/feeds/keyword-suggestions: keywords to add while building a feed.
// It lives under /feeds because that is what it is for, though the data comes from the global article
// pool (articles are public, so nothing is user-scoped here).
//
// Query params:
//   - keywords: the keywords already picked, comma-separated (optional). When present, the response
//     favours keywords that co-occur with them ("related"); when absent — or when nothing co-occurs —
//     it returns the most popular keywords instead ("popular"). The response always echoes which
//     strategy was used, so the client can label the list and the fallback is observable.
//   - limit: how many suggestions to return (default 10, clamped to [1, 50]).
//
// This is a ranked indicator, not a record listing, so it is not paginated (conventions.md carves out
// this exception, like check-for-new-articles). windowDays bounds the "popular" side to recent
// articles; it is resolved to a concrete "since" per request.
func SuggestKeywords(ctrl controllers.ArticleControllerInterface, runTx controllers.TransactionRunner, windowDays int) fiber.Handler {
	return func(c fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())

		selected, err := parseSelectedKeywords(c.Query("keywords"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		limit := parseSuggestionLimit(c.Query("limit"))
		since := suggestionSince(windowDays)

		var suggestions []schemas.KeywordSuggestion
		var strategy string
		err = runTx(c.Context(), func(q db.Querier) error {
			var e error
			suggestions, strategy, e = ctrl.SuggestKeywords(c.Context(), q, selected, since, int32(limit))
			return e
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build keyword suggestions"})
		}

		return c.JSON(schemas.KeywordSuggestionsResponse{Strategy: strategy, Suggestions: suggestions})
	}
}

// parseSelectedKeywords normalizes the comma-separated `keywords` query into the clean slice the
// controller expects: trimmed, lowercased, empties dropped, duplicates removed. An empty or absent
// value means "nothing picked yet" (empty slice, not an error). More than a feed's worth of keywords
// is rejected — the picks can never exceed what a feed could hold, and it also caps the size of the
// `?|` array the query builds.
func parseSelectedKeywords(raw string) ([]string, error) {
	out := []string{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}

	seen := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		kw := strings.ToLower(strings.TrimSpace(part))
		if kw == "" {
			continue
		}
		if _, dup := seen[kw]; dup {
			continue
		}
		seen[kw] = struct{}{}
		out = append(out, kw)
	}

	if len(out) > schemas.FeedKeywordsMax {
		return nil, fmt.Errorf("too many keywords: at most %d are allowed", schemas.FeedKeywordsMax)
	}
	return out, nil
}

// parseSuggestionLimit reads the optional `limit`. Like pagination.ParseParams it never errors: a
// missing or unparseable value falls back to the default, and the value is clamped to [1, max].
func parseSuggestionLimit(raw string) int {
	limit := schemas.KeywordSuggestionsDefaultLimit
	if n, err := strconv.Atoi(raw); err == nil {
		limit = n
	}
	if limit < 1 {
		limit = 1
	}
	if limit > schemas.KeywordSuggestionsMaxLimit {
		limit = schemas.KeywordSuggestionsMaxLimit
	}
	return limit
}

// suggestionSince turns the configured window (in days) into the lower bound for the "popular" query.
// A negative window disables the bound (everything since the epoch); otherwise it is now minus the
// window. Computed per request so "recent" tracks the wall clock. All dates are UTC.
func suggestionSince(windowDays int) time.Time {
	if windowDays < 0 {
		return time.Unix(0, 0).UTC()
	}
	return time.Now().UTC().AddDate(0, 0, -windowDays)
}
