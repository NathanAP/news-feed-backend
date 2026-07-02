package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTranslation_Object(t *testing.T) {
	out, err := ParseTranslation(`{"title":"Título","content":"<p>Corpo</p>","keywords":["rock","metal"]}`)
	require.NoError(t, err)
	assert.Equal(t, "Título", out.Title)
	assert.Equal(t, "<p>Corpo</p>", out.Content)
	assert.Equal(t, []string{"rock", "metal"}, out.Keywords)
}

func TestParseTranslation_StripsCodeFence(t *testing.T) {
	raw := "```json\n{\"title\":\"T\",\"content\":\"<p>C</p>\",\"keywords\":[]}\n```"
	out, err := ParseTranslation(raw)
	require.NoError(t, err)
	assert.Equal(t, "T", out.Title)
	assert.Empty(t, out.Keywords)
}

func TestParseTranslation_TrimsEmptyKeywords(t *testing.T) {
	out, err := ParseTranslation(`{"title":"T","content":"C","keywords":["rock","  ","metal",""]}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"rock", "metal"}, out.Keywords)
}

func TestParseTranslation_MissingTitle(t *testing.T) {
	_, err := ParseTranslation(`{"title":"","content":"C","keywords":[]}`)
	assert.ErrorIs(t, err, ErrInvalidTranslation)
}

func TestParseTranslation_MissingContent(t *testing.T) {
	_, err := ParseTranslation(`{"title":"T","content":"   ","keywords":[]}`)
	assert.ErrorIs(t, err, ErrInvalidTranslation)
}

func TestParseTranslation_InvalidJSON(t *testing.T) {
	_, err := ParseTranslation(`not json`)
	assert.ErrorIs(t, err, ErrInvalidTranslation)
}
