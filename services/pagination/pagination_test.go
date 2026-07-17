package pagination_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nathanap/news-feed-backend/services/pagination"
)

func TestParseParams_Defaults(t *testing.T) {
	p := pagination.ParseParams("", "")
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.PageSize)
}

func TestParseParams_Invalid(t *testing.T) {
	p := pagination.ParseParams("abc", "xyz")
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.PageSize)
}

func TestParseParams_Clamps(t *testing.T) {
	// Below the minimum.
	p := pagination.ParseParams("0", "0")
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 1, p.PageSize)

	// page has no upper bound; page_size caps at 100.
	p = pagination.ParseParams("999", "500")
	assert.Equal(t, 999, p.Page)
	assert.Equal(t, 100, p.PageSize)
}

func TestParseParams_Negative(t *testing.T) {
	p := pagination.ParseParams("-5", "-3")
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 1, p.PageSize)
}

// Limit/Offset are what the SQL LIMIT/OFFSET is built from, so an error here silently serves the
// wrong slice of the table.
func TestParams_LimitAndOffset(t *testing.T) {
	tests := []struct {
		name       string
		params     pagination.Params
		wantLimit  int32
		wantOffset int32
	}{
		{name: "first page starts at zero", params: pagination.Params{Page: 1, PageSize: 20}, wantLimit: 20, wantOffset: 0},
		{name: "second page skips one page", params: pagination.Params{Page: 2, PageSize: 20}, wantLimit: 20, wantOffset: 20},
		{name: "offset follows page size", params: pagination.Params{Page: 4, PageSize: 5}, wantLimit: 5, wantOffset: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantLimit, tt.params.Limit())
			assert.Equal(t, tt.wantOffset, tt.params.Offset())
		})
	}
}

func TestBuildResponse_FirstPage(t *testing.T) {
	// The query returned this page's rows; the count query reported 5 matches in total.
	resp := pagination.BuildResponse([]int{1, 2}, 5, pagination.Params{Page: 1, PageSize: 2})

	assert.Equal(t, []int{1, 2}, resp.Docs)
	assert.Equal(t, 1, resp.Pagination.ActualPage)
	assert.Equal(t, 3, resp.Pagination.TotalPages) // ceil(5/2)
	assert.Equal(t, 2, resp.Pagination.ActualCount)
	assert.Equal(t, int64(5), resp.Pagination.TotalCount)
	assert.True(t, resp.Pagination.HasNextPage)
	assert.False(t, resp.Pagination.HasPreviousPage)
}

func TestBuildResponse_LastPagePartial(t *testing.T) {
	resp := pagination.BuildResponse([]int{5}, 5, pagination.Params{Page: 3, PageSize: 2})

	assert.Equal(t, []int{5}, resp.Docs)
	assert.Equal(t, 1, resp.Pagination.ActualCount)
	assert.False(t, resp.Pagination.HasNextPage)
	assert.True(t, resp.Pagination.HasPreviousPage)
}

// A page past the end is not an error (PROJECT.md): the query yields no rows, but the count still
// reports the truth, so actual_page stays as requested while total_pages reflects reality. This is
// the case a COUNT(*) OVER() could not serve — with no rows there would be no window to read the
// total from.
func TestBuildResponse_OutOfRangeReturnsEmpty(t *testing.T) {
	resp := pagination.BuildResponse([]int{}, 5, pagination.Params{Page: 5, PageSize: 2})

	assert.Empty(t, resp.Docs)
	assert.NotNil(t, resp.Docs) // must marshal to [] not null
	assert.Equal(t, 5, resp.Pagination.ActualPage)
	assert.Equal(t, 3, resp.Pagination.TotalPages)
	assert.Equal(t, 0, resp.Pagination.ActualCount)
	assert.Equal(t, int64(5), resp.Pagination.TotalCount)
	assert.False(t, resp.Pagination.HasNextPage)
	assert.True(t, resp.Pagination.HasPreviousPage)
}

func TestBuildResponse_Empty(t *testing.T) {
	resp := pagination.BuildResponse([]int{}, 0, pagination.Params{Page: 1, PageSize: 20})

	assert.Empty(t, resp.Docs)
	assert.Equal(t, 0, resp.Pagination.TotalPages)
	assert.Equal(t, int64(0), resp.Pagination.TotalCount)
	assert.False(t, resp.Pagination.HasNextPage)
	assert.False(t, resp.Pagination.HasPreviousPage)
}

// A nil slice (what a query returns when it matches nothing) must still marshal to [], never null.
func TestBuildResponse_NilDocs(t *testing.T) {
	resp := pagination.BuildResponse[int](nil, 0, pagination.Params{Page: 1, PageSize: 20})

	assert.NotNil(t, resp.Docs)
	assert.Empty(t, resp.Docs)
}
