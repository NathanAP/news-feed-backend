package ai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/ai"
)

func TestPassthroughTreater_ReturnsContentUnchanged(t *testing.T) {
	treater := ai.NewPassthroughTreater()

	content := `<p>Original <a href="https://x.test">link</a> kept as-is.</p>`
	out, err := treater.Treat(context.Background(), "Some title", content)

	require.NoError(t, err)
	// The passthrough performs no cleanup of its own: the content comes back byte-for-byte. Any
	// HTML whitelisting is the sanitize layer's job, applied downstream in the wiring.
	assert.Equal(t, content, out)
}

func TestPassthroughTreater_EmptyContentIsNotAnError(t *testing.T) {
	treater := ai.NewPassthroughTreater()

	out, err := treater.Treat(context.Background(), "title", "")

	require.NoError(t, err)
	assert.Equal(t, "", out)
}
