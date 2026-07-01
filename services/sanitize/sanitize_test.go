package sanitize_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/sanitize"
)

func TestSanitize_KeepsBasicTags(t *testing.T) {
	out := sanitize.Sanitize(`<p>Hello <strong>world</strong> and <em>more</em></p>`)
	assert.Equal(t, `<p>Hello <strong>world</strong> and <em>more</em></p>`, out)
}

func TestSanitize_StripsLinksImagesKeepingText(t *testing.T) {
	out := sanitize.Sanitize(`<p>see <a href="https://x.com">this</a></p><img src="y.png">`)
	assert.NotContains(t, out, "<a")
	assert.NotContains(t, out, "<img")
	assert.NotContains(t, out, "https://x.com")
	assert.Contains(t, out, "this") // anchor text is preserved
}

func TestSanitize_StripsClassAndStyle(t *testing.T) {
	out := sanitize.Sanitize(`<p class="foo" style="color:red">text</p>`)
	assert.Equal(t, `<p>text</p>`, out)
}

func TestSanitize_DropsScriptAndStyleContent(t *testing.T) {
	out := sanitize.Sanitize(`<p>ok</p><script>alert('xss')</script><style>.a{color:red}</style>`)
	assert.NotContains(t, out, "alert")
	assert.NotContains(t, out, "color:red")
	assert.Contains(t, out, "<p>ok</p>")
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

// stubTreater lets us test the sanitizing decorator.
type stubTreater struct{ out string }

func (s stubTreater) Treat(context.Context, string, string) (string, error) { return s.out, nil }

func TestNewTreater_SanitizesOutput(t *testing.T) {
	treater := sanitize.NewTreater(stubTreater{out: `<p>hi <a href="http://x">x</a></p>`})
	out, err := treater.Treat(context.Background(), "title", "content")
	require.NoError(t, err)
	assert.False(t, strings.Contains(out, "<a"), "link must be stripped by the decorator")
	assert.Contains(t, out, "<p>hi ")
}
