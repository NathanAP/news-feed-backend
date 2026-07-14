package embedtreatment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/embedtreatment"
)

func TestTreat_ConvertsInstagramToLink(t *testing.T) {
	content := `<figure><div class="wp-block-embed__wrapper">` +
		`<blockquote class="instagram-media" data-instgrm-permalink="https://www.instagram.com/p/DaS_XhAH9NT/?utm_source=ig_embed&amp;utm_campaign=loading" data-instgrm-version="14"><div>caption</div></blockquote>` +
		`<script async src="//platform.instagram.com/embeds.js"></script></div></figure>`

	out, err := embedtreatment.Treat(content, "", false)
	require.NoError(t, err)
	assert.Contains(t, out, `<a href="https://www.instagram.com/p/DaS_XhAH9NT/">https://www.instagram.com/p/DaS_XhAH9NT/</a>`, "blockquote becomes a clean link")
	assert.NotContains(t, out, "instagram-media", "the blockquote is gone")
	assert.NotContains(t, out, "data-instgrm-permalink")
	// The loader script is not embedtreatment's job to remove (sanitize drops it), but it must survive
	// this step untouched so the pipeline order is clear.
	assert.Contains(t, out, "platform.instagram.com/embeds.js")
}

func TestTreat_FallsBackToInnerAnchor(t *testing.T) {
	// No data-instgrm-permalink: use the first inner <a> pointing at a post, cleaned of query string.
	content := `<blockquote class="instagram-media"><a href="https://www.instagram.com/p/ABC123/?utm_source=ig_embed">view</a></blockquote>`
	out, err := embedtreatment.Treat(content, "", false)
	require.NoError(t, err)
	assert.Contains(t, out, `<a href="https://www.instagram.com/p/ABC123/">https://www.instagram.com/p/ABC123/</a>`)
	assert.NotContains(t, out, "instagram-media")
}

func TestTreat_NoEmbedReturnsOriginal(t *testing.T) {
	content := `<p>just text with a <a href="https://x.com">link</a></p>`
	out, err := embedtreatment.Treat(content, "", false)
	require.NoError(t, err)
	assert.Equal(t, content, out) // nothing to convert → original preserved verbatim
}

func TestTreat_MissingPermalinkLeavesNode(t *testing.T) {
	// instagram-media blockquote with no permalink anywhere: it is not converted (left for sanitize),
	// and the step must not crash.
	content := `<blockquote class="instagram-media"><div>caption only</div></blockquote>`
	out, err := embedtreatment.Treat(content, "", false)
	require.NoError(t, err)
	assert.Contains(t, out, "caption only")
	assert.NotContains(t, out, "<a ") // nothing was turned into a link
}

func TestTreat_LeavesRegularBlockquote(t *testing.T) {
	content := `<blockquote>A normal quote</blockquote>`
	out, err := embedtreatment.Treat(content, "", false)
	require.NoError(t, err)
	assert.Equal(t, content, out) // non-instagram blockquote is untouched
}

func TestTreat_RewritesTwitchParent(t *testing.T) {
	// A Twitch iframe carrying the source's parent must be rewritten to our client's host so it plays.
	content := `<iframe src="https://player.twitch.tv/?video=123&amp;parent=wikimetal.com.br" allowfullscreen="true"></iframe>`
	out, err := embedtreatment.Treat(content, "https://app.example.com", false)
	require.NoError(t, err)
	assert.Contains(t, out, "parent=app.example.com", "parent set to the client host")
	assert.NotContains(t, out, "parent=wikimetal.com.br", "the source's parent is dropped")
	assert.Contains(t, out, "video=123", "other params are preserved")
}

func TestTreat_TwitchParentUsesHostnameWithoutPort(t *testing.T) {
	content := `<iframe src="https://player.twitch.tv/?channel=x"></iframe>`
	out, err := embedtreatment.Treat(content, "http://localhost:5173", false)
	require.NoError(t, err)
	assert.Contains(t, out, "parent=localhost", "port is dropped from the parent host")
	assert.NotContains(t, out, "parent=localhost%3A5173")
}

func TestTreat_TwitchClipParentRewritten(t *testing.T) {
	content := `<iframe src="https://clips.twitch.tv/embed?clip=Foo&amp;parent=old.com"></iframe>`
	out, err := embedtreatment.Treat(content, "https://app.example.com", false)
	require.NoError(t, err)
	assert.Contains(t, out, "parent=app.example.com")
}

func TestTreat_LeavesYouTubeAndOtherIframes(t *testing.T) {
	content := `<iframe src="https://www.youtube.com/embed/abc?rel=1"></iframe>`
	out, err := embedtreatment.Treat(content, "https://app.example.com", false)
	require.NoError(t, err)
	assert.Equal(t, content, out) // YouTube needs no parent → untouched (verbatim)
}

func TestTreat_TwitchSkippedWithoutClientURL(t *testing.T) {
	content := `<iframe src="https://player.twitch.tv/?video=123&amp;parent=old.com"></iframe>`
	out, err := embedtreatment.Treat(content, "", false)
	require.NoError(t, err)
	assert.Equal(t, content, out) // no client host known → left as-is (verbatim)
}
