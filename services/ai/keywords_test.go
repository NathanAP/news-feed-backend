package ai

import (
	"errors"
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
	many := `["k1","k2","k3","k4","k5","k6","k7","k8","k9","k10","k11","k12","k13","k14","k15","k16","k17","k18","k19","k20","k21"]`
	_, err := ParseKeywords(many)
	assert.ErrorIs(t, err, ErrInvalidKeywords)
}

func TestParseKeywords_InvalidJSON(t *testing.T) {
	_, err := ParseKeywords(`not json at all`)
	assert.True(t, errors.Is(err, ErrInvalidKeywords))
}
