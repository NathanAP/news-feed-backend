// Package pagination provides the application-wide pagination contract used by every collection
// endpoint (GET /v1/{model}/). It parses the page/page_size query params and slices an already
// materialized, filtered slice into the response envelope defined in PROJECT.md. Slicing happens in
// memory: the list endpoints already load and filter their rows in Go, so pagination stays at the
// same layer (revisit with SQL LIMIT/OFFSET if the dataset outgrows this, e.g. after Postgres).
package pagination

import "strconv"

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
	ActualPage      int  `json:"actual_page"`
	TotalPages      int  `json:"total_pages"`
	ActualCount     int  `json:"actual_count"`
	TotalCount      int  `json:"total_count"`
	HasNextPage     bool `json:"has_next_page"`
	HasPreviousPage bool `json:"has_previous_page"`
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

// Paginate slices items to the requested page and builds the response envelope. A page beyond the
// available range yields an empty docs list (not an error): actual_page stays at what was requested
// while total_pages reflects reality (PROJECT.md). Docs is always non-nil so it marshals to [].
func Paginate[T any](items []T, p Params) Response[T] {
	total := len(items)

	totalPages := 0
	if total > 0 {
		totalPages = (total + p.PageSize - 1) / p.PageSize
	}

	offset := (p.Page - 1) * p.PageSize
	docs := make([]T, 0, p.PageSize)
	if offset < total {
		end := offset + p.PageSize
		if end > total {
			end = total
		}
		docs = append(docs, items[offset:end]...)
	}

	return Response[T]{
		Docs: docs,
		Pagination: Meta{
			ActualPage:      p.Page,
			TotalPages:      totalPages,
			ActualCount:     len(docs),
			TotalCount:      total,
			HasNextPage:     p.Page < totalPages,
			HasPreviousPage: p.Page > 1,
		},
	}
}
