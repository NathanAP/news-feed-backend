package sanitize_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nathanap/news-feed-backend/services/sanitize"
)

func TestSanitize_KeepsBasicTags(t *testing.T) {
	out := sanitize.Sanitize(`<p>Hello <strong>world</strong> and <em>more</em></p>`)
	assert.Equal(t, `<p>Hello <strong>world</strong> and <em>more</em></p>`, out)
}

func TestSanitize_KeepsSafeLinks(t *testing.T) {
	out := sanitize.Sanitize(`<p>see <a href="https://x.com/post">this</a></p>`)
	assert.Contains(t, out, `href="https://x.com/post"`)
	assert.Contains(t, out, `rel="nofollow"`) // links are tagged nofollow
	assert.Contains(t, out, "this")
}

func TestSanitize_KeepsImages(t *testing.T) {
	out := sanitize.Sanitize(`<p><img src="https://x.com/a.png" alt="a" width="100" height="50"></p>`)
	assert.Contains(t, out, `src="https://x.com/a.png"`)
	assert.Contains(t, out, `alt="a"`)
}

func TestSanitize_DropsUnsafeURLSchemes(t *testing.T) {
	out := sanitize.Sanitize(`<a href="javascript:alert(1)">x</a>`)
	assert.NotContains(t, out, "javascript:")
	assert.Contains(t, out, "x") // anchor text preserved, href dropped
}

func TestSanitize_StripsClassStyleAndEventHandlers(t *testing.T) {
	out := sanitize.Sanitize(`<p class="foo" style="color:red" onclick="hack()">text</p>`)
	assert.Equal(t, `<p>text</p>`, out)
}

func TestSanitize_DropsScriptStyleAndIframe(t *testing.T) {
	out := sanitize.Sanitize(`<p>ok</p><script>alert('xss')</script><style>.a{color:red}</style><iframe src="https://evil.com"></iframe>`)
	assert.NotContains(t, out, "alert")
	assert.NotContains(t, out, "color:red")
	assert.NotContains(t, out, "<iframe")
	assert.NotContains(t, out, "evil.com")
	assert.Contains(t, out, "<p>ok</p>")
}

// TestSanitize_DropsInstagramScriptEmbed covers the script-based embed case (Instagram): the
// blockquote text survives but the loader <script> is dropped entirely (never executed client-side).
func TestSanitize_DropsInstagramScriptEmbed(t *testing.T) {
	out := sanitize.Sanitize(`<blockquote class="instagram-media">post</blockquote><script async src="//platform.instagram.com/embeds.js"></script>`)
	assert.NotContains(t, out, "<script")
	assert.NotContains(t, out, "instagram.com/embeds.js")
	assert.Contains(t, out, "post")
}

func TestSanitize_KeepsRicherBlockTags(t *testing.T) {
	out := sanitize.Sanitize(`<figure><img src="https://x.com/a.png" alt="a"><figcaption>cap</figcaption></figure><ul><li>one</li></ul>`)
	assert.Contains(t, out, "<figure>")
	assert.Contains(t, out, "<figcaption>cap</figcaption>")
	assert.Contains(t, out, "<li>one</li>")
}

func TestSanitize_HandlesFullHTMLDocument(t *testing.T) {
	doc := `<html><head><title>t</title><style>.x{}</style></head><body><p>Body <strong>text</strong></p></body></html>`
	out := sanitize.Sanitize(doc)
	assert.NotContains(t, out, "<html")
	assert.NotContains(t, out, "<body")
	assert.NotContains(t, out, "<head")
	assert.NotContains(t, out, ".x{}") // head/style content dropped
	assert.Contains(t, out, "<p>Body <strong>text</strong></p>")
}
