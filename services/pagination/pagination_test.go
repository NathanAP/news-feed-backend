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

func TestPaginate_FirstPage(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	resp := pagination.Paginate(items, pagination.Params{Page: 1, PageSize: 2})

	assert.Equal(t, []int{1, 2}, resp.Docs)
	assert.Equal(t, 1, resp.Pagination.ActualPage)
	assert.Equal(t, 3, resp.Pagination.TotalPages) // ceil(5/2)
	assert.Equal(t, 2, resp.Pagination.ActualCount)
	assert.Equal(t, 5, resp.Pagination.TotalCount)
	assert.True(t, resp.Pagination.HasNextPage)
	assert.False(t, resp.Pagination.HasPreviousPage)
}

func TestPaginate_LastPagePartial(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	resp := pagination.Paginate(items, pagination.Params{Page: 3, PageSize: 2})

	assert.Equal(t, []int{5}, resp.Docs)
	assert.Equal(t, 1, resp.Pagination.ActualCount)
	assert.False(t, resp.Pagination.HasNextPage)
	assert.True(t, resp.Pagination.HasPreviousPage)
}

func TestPaginate_OutOfRangeReturnsEmpty(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	resp := pagination.Paginate(items, pagination.Params{Page: 5, PageSize: 2})

	// Page beyond the data: empty docs, but actual_page stays as requested (PROJECT.md).
	assert.Empty(t, resp.Docs)
	assert.NotNil(t, resp.Docs) // must marshal to [] not null
	assert.Equal(t, 5, resp.Pagination.ActualPage)
	assert.Equal(t, 3, resp.Pagination.TotalPages)
	assert.Equal(t, 0, resp.Pagination.ActualCount)
	assert.Equal(t, 5, resp.Pagination.TotalCount)
	assert.False(t, resp.Pagination.HasNextPage)
	assert.True(t, resp.Pagination.HasPreviousPage)
}

func TestPaginate_Empty(t *testing.T) {
	resp := pagination.Paginate([]int{}, pagination.Params{Page: 1, PageSize: 20})

	assert.Empty(t, resp.Docs)
	assert.Equal(t, 0, resp.Pagination.TotalPages)
	assert.Equal(t, 0, resp.Pagination.TotalCount)
	assert.False(t, resp.Pagination.HasNextPage)
	assert.False(t, resp.Pagination.HasPreviousPage)
}
