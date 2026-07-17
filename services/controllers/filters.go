package controllers

import (
	"database/sql"
	"time"

	"github.com/nathanap/news-feed-backend/services/pagination"
	"github.com/nathanap/news-feed-backend/services/utctime"
)

// This file holds the list filters shared by the collection endpoints and the helpers that turn an
// "absent" filter into the NULL the queries expect.
//
// Every list filter is optional and every query spells that as `sqlc.narg(x) IS NULL OR <test>`, so
// a filter that was not requested must reach the query as a NULL parameter rather than as a zero
// value: passing an empty string would filter for rows whose column contains "" (all of them, by
// luck) and passing a false bool would filter for is_read = false (silently wrong). The nullable*
// helpers below are the single place that mapping happens.

// ListArticlesFilter narrows GET /v1/articles. URL is an optional case-insensitive substring match
// against url_original; empty means the filter is not applied.
type ListArticlesFilter struct {
	URL  string
	Page pagination.Params
}

// ListSourcesFilter narrows GET /v1/sources. URL and Name are independent optional
// case-insensitive substring matches; empty means the filter is not applied.
type ListSourcesFilter struct {
	URL  string
	Name string
	Page pagination.Params
}

// ListFeedsFilter narrows GET /v1/feeds. Name is an optional case-insensitive substring match;
// empty means the filter is not applied. The user scope is not here: it is a mandatory argument of
// the List method, never an optional filter, so it cannot be forgotten at a call site.
type ListFeedsFilter struct {
	Name string
	Page pagination.Params
}

// ListFeedArticlesFilter narrows GET /v1/feeds/{id}/articles. All fields are optional (nil = not
// applied): IsRead matches the junction's read state, and PeriodStartingAt / PeriodEndingAt bound
// the article's created_at, independently and inclusive on both ends. The times are expected in UTC
// (dates are UTC across the application); the handler parses and normalizes them.
type ListFeedArticlesFilter struct {
	IsRead           *bool
	PeriodStartingAt *time.Time
	PeriodEndingAt   *time.Time
	Page             pagination.Params
}

// nullableString maps an absent (empty) filter to NULL and any other value to itself.
func nullableString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// nullableBool maps an absent (nil) filter to NULL. A non-nil false is a real filter (match unread
// rows), which is exactly why the filter is a pointer rather than a plain bool.
func nullableBool(value *bool) sql.NullBool {
	if value == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *value, Valid: true}
}

// nullableTime maps an absent (nil) bound to NULL, normalizing a present one to UTC.
func nullableTime(value *time.Time) utctime.NullTime {
	if value == nil {
		return utctime.NullTime{}
	}
	return utctime.NewNull(*value)
}
