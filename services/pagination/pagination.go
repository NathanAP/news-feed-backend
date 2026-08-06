// Package pagination provides the application-wide pagination contract used by every collection
// endpoint (GET /v1/{model}/). It parses the page/page_size query params into a normalized request
// that the SQL layer turns into LIMIT/OFFSET, and builds the response envelope defined in
// PROJECT.md around a page of rows plus the total count reported by the database.
//
// Nothing is filtered or sliced in Go: each list query applies its filters and its LIMIT/OFFSET in
// SQL and a sibling COUNT query (same filters) reports the total, so a handler only ever holds the
// rows of the page it is about to serve.
package pagination

import (
	"math"
	"strconv"
)

const (
	DefaultPage     = 1
	MinPage         = 1
	DefaultPageSize = 20
	MinPageSize     = 1
	MaxPageSize     = 100
)

// Params holds the normalized (clamped) pagination request.
type Params struct {
	Page     int
	PageSize int
}

// Meta is the pagination metadata returned alongside the page of records.
type Meta struct {
	ActualPage      int   `json:"actual_page"`
	TotalPages      int   `json:"total_pages"`
	ActualCount     int   `json:"actual_count"`
	TotalCount      int64 `json:"total_count"`
	HasNextPage     bool  `json:"has_next_page"`
	HasPreviousPage bool  `json:"has_previous_page"`
}

// Response is the paginated envelope: the page of records plus the metadata.
type Response[T any] struct {
	Docs       []T  `json:"docs"`
	Pagination Meta `json:"pagination"`
}

// ParseParams reads the raw `page` and `page_size` query values and normalizes them: missing or
// invalid values fall back to the defaults, and both are clamped to their allowed ranges (page >= 1;
// page_size in [1, 100]). It never errors — a malformed query just yields a sane request.
func ParseParams(pageRaw, pageSizeRaw string) Params {
	page := DefaultPage
	if n, err := strconv.Atoi(pageRaw); err == nil {
		page = n
	}
	if page < MinPage {
		page = MinPage
	}

	pageSize := DefaultPageSize
	if n, err := strconv.Atoi(pageSizeRaw); err == nil {
		pageSize = n
	}
	if pageSize < MinPageSize {
		pageSize = MinPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return Params{Page: page, PageSize: pageSize}
}

// Limit is the row count to fetch, for the query's LIMIT.
func (p Params) Limit() int32 {
	return int32(p.PageSize)
}

// Offset is how many rows to skip to reach the requested page, for the query's OFFSET.
//
// It saturates at MaxInt32 instead of overflowing. ParseParams clamps page from below but puts no
// ceiling on it (by design: an out-of-range page is legal and must echo back in actual_page), so
// `?page=200000000` reaches here — and the old `int32((page-1)*pageSize)` wrapped that to a NEGATIVE
// offset, which Postgres rejects outright, turning a query param into a 500. Saturating keeps the
// documented behaviour instead: a page far past the data is simply a page with no rows.
//
// The comparison is done by division rather than by multiplying first, so the guard itself cannot
// overflow. PageSize is always >= 1 (ParseParams clamps it), so the division is safe.
func (p Params) Offset() int32 {
	pagesToSkip := int64(p.Page) - 1
	if pagesToSkip > int64(math.MaxInt32)/int64(p.PageSize) {
		return math.MaxInt32
	}
	return int32(pagesToSkip * int64(p.PageSize))
}

// BuildResponse wraps a page of rows and the total count of matching rows into the response
// envelope. docs is what the query returned for this page; totalCount comes from the sibling COUNT
// query, NOT from len(docs) — that is what lets an out-of-range page report the truth: it yields no
// rows, so actual_page stays at what was requested while total_pages reflects reality (PROJECT.md).
// Docs is normalized to non-nil so it always marshals to [] instead of null.
func BuildResponse[T any](docs []T, totalCount int64, p Params) Response[T] {
	if docs == nil {
		docs = []T{}
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = int((totalCount + int64(p.PageSize) - 1) / int64(p.PageSize))
	}

	return Response[T]{
		Docs: docs,
		Pagination: Meta{
			ActualPage:      p.Page,
			TotalPages:      totalPages,
			ActualCount:     len(docs),
			TotalCount:      totalCount,
			HasNextPage:     p.Page < totalPages,
			HasPreviousPage: p.Page > 1,
		},
	}
}
