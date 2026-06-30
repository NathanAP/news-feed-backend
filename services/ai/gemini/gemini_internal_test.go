package gemini

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/ai"
)

func TestParseKeywords_Valid(t *testing.T) {
	out, err := parseKeywords(`["alpha","beta","gamma","delta","epsilon"]`)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma", "delta", "epsilon"}, out)
}

func TestParseKeywords_StripsCodeFence(t *testing.T) {
	raw := "```json\n[\"alpha\",\"beta\",\"gamma\",\"delta\",\"epsilon\"]\n```"
	out, err := parseKeywords(raw)
	require.NoError(t, err)
	assert.Len(t, out, 5)
}

func TestParseKeywords_DeduplicatesCaseInsensitive(t *testing.T) {
	// 8 raw items, 2 are case-insensitive duplicates → 6 distinct, still within 5-20.
	out, err := parseKeywords(`["Rock","rock","metal","Metal","music","concert","tour","band"]`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Rock", "metal", "music", "concert", "tour", "band"}, out)
}

func TestParseKeywords_TooFew(t *testing.T) {
	_, err := parseKeywords(`["a","b","c"]`)
	assert.ErrorIs(t, err, ai.ErrInvalidKeywords)
}

func TestParseKeywords_TooMany(t *testing.T) {
	many := `["k1","k2","k3","k4","k5","k6","k7","k8","k9","k10","k11","k12","k13","k14","k15","k16","k17","k18","k19","k20","k21"]`
	_, err := parseKeywords(many)
	assert.ErrorIs(t, err, ai.ErrInvalidKeywords)
}

func TestParseKeywords_InvalidJSON(t *testing.T) {
	_, err := parseKeywords(`not json at all`)
	assert.True(t, errors.Is(err, ai.ErrInvalidKeywords))
}
