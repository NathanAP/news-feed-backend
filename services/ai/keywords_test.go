package ai

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKeywords_Array(t *testing.T) {
	out, err := ParseKeywords(`["alpha","beta","gamma","delta","epsilon"]`)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma", "delta", "epsilon"}, out)
}

func TestParseKeywords_Object(t *testing.T) {
	out, err := ParseKeywords(`{"keywords":["alpha","beta","gamma","delta","epsilon"]}`)
	require.NoError(t, err)
	assert.Len(t, out, 5)
}

func TestParseKeywords_StripsCodeFence(t *testing.T) {
	raw := "```json\n[\"alpha\",\"beta\",\"gamma\",\"delta\",\"epsilon\"]\n```"
	out, err := ParseKeywords(raw)
	require.NoError(t, err)
	assert.Len(t, out, 5)
}

func TestParseKeywords_LowercasesAndDeduplicates(t *testing.T) {
	out, err := ParseKeywords(`["Rock","rock","metal","Metal","music","concert","tour","band"]`)
	require.NoError(t, err)
	assert.Equal(t, []string{"rock", "metal", "music", "concert", "tour", "band"}, out)
}

func TestParseKeywords_TooFew(t *testing.T) {
	_, err := ParseKeywords(`["a","b","c"]`)
	assert.ErrorIs(t, err, ErrInvalidKeywords)
}

func TestParseKeywords_TooMany(t *testing.T) {
	// Over the 30-keyword max (0.36.2): 31 distinct keywords.
	items := make([]string, 31)
	for i := range items {
		items[i] = fmt.Sprintf("%q", fmt.Sprintf("k%d", i))
	}
	_, err := ParseKeywords("[" + strings.Join(items, ",") + "]")
	assert.ErrorIs(t, err, ErrInvalidKeywords)
}

func TestParseKeywords_InvalidJSON(t *testing.T) {
	_, err := ParseKeywords(`not json at all`)
	assert.True(t, errors.Is(err, ErrInvalidKeywords))
}
