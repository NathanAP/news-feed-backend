package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScore_Object(t *testing.T) {
	score, err := ParseScore(`{"score": 82}`)
	require.NoError(t, err)
	assert.Equal(t, 82, score)
}

func TestParseScore_BareInteger(t *testing.T) {
	score, err := ParseScore(`73`)
	require.NoError(t, err)
	assert.Equal(t, 73, score)
}

func TestParseScore_StripsCodeFence(t *testing.T) {
	score, err := ParseScore("```json\n{\"score\": 55}\n```")
	require.NoError(t, err)
	assert.Equal(t, 55, score)
}

func TestParseScore_Boundaries(t *testing.T) {
	zero, err := ParseScore(`{"score": 0}`)
	require.NoError(t, err)
	assert.Equal(t, 0, zero)

	hundred, err := ParseScore(`{"score": 100}`)
	require.NoError(t, err)
	assert.Equal(t, 100, hundred)
}

func TestParseScore_OutOfRange(t *testing.T) {
	_, err := ParseScore(`{"score": 140}`)
	assert.ErrorIs(t, err, ErrInvalidScore)

	_, err = ParseScore(`{"score": -5}`)
	assert.ErrorIs(t, err, ErrInvalidScore)
}

func TestParseScore_Invalid(t *testing.T) {
	_, err := ParseScore(`no score here`)
	assert.ErrorIs(t, err, ErrInvalidScore)
}
