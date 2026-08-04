package outboundlinks

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seqIDs returns a newID func that hands out id-1, id-2, ... deterministically, so tests can assert on
// the exact ids written into the body and returned as links.
func seqIDs() func() (string, error) {
	n := 0
	return func() (string, error) {
		n++
		return fmt.Sprintf("id-%d", n), nil
	}
}

func TestAssign_NoAnchors(t *testing.T) {
	content := `<p>plain text, no links</p>`
	out, links, err := Assign(content, "https://client.app", seqIDs())
	require.NoError(t, err)
	assert.Equal(t, content, out, "content without anchors is returned unchanged")
	assert.Nil(t, links)
}

func TestAssign_ExternalLinkStoredLiterally(t *testing.T) {
	out, links, err := Assign(`<a href="https://ext.com/z">x</a>`, "https://client.app", seqIDs())
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "https://ext.com/z", links[0].Href, "external href stored literally")
	assert.Contains(t, out, `href="id-1"`, "body anchor carries the outbound id, not the URL")
	assert.NotContains(t, out, "https://ext.com/z")
}

func TestAssign_InternalLinkStoredAsToken(t *testing.T) {
	out, links, err := Assign(`<a href="https://client.app/articles/abc">x</a>`, "https://client.app", seqIDs())
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "{CLIENT_URL}/articles/abc", links[0].Href, "internal href stored as the CLIENT_URL token")
	assert.Contains(t, out, `href="id-1"`)
}

func TestAssign_EmptyClientURLStoresEverythingLiterally(t *testing.T) {
	// With no client URL, HasPrefix(href, "") must NOT turn every href into a token — external links
	// would be corrupted. toStored guards on clientURL != "".
	_, links, err := Assign(`<a href="https://ext.com/z">x</a>`, "", seqIDs())
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "https://ext.com/z", links[0].Href)
}

func TestAssign_DuplicateHrefSharesOneID(t *testing.T) {
	out, links, err := Assign(`<a href="https://ext.com/z">a</a><a href="https://ext.com/z">b</a>`, "https://client.app", seqIDs())
	require.NoError(t, err)
	require.Len(t, links, 1, "one row per distinct href")
	assert.Equal(t, 2, strings.Count(out, `href="id-1"`), "both anchors point at the shared id")
}

func TestAssign_MixedInternalAndExternal(t *testing.T) {
	content := `<p>see <a href="https://client.app/articles/abc">ours</a> and <a href="https://ext.com/z">ext</a></p>`
	out, links, err := Assign(content, "https://client.app", seqIDs())
	require.NoError(t, err)
	require.Len(t, links, 2)
	assert.Equal(t, "{CLIENT_URL}/articles/abc", links[0].Href)
	assert.Equal(t, "https://ext.com/z", links[1].Href)
	assert.Contains(t, out, `href="id-1"`)
	assert.Contains(t, out, `href="id-2"`)
}

func TestAssign_PropagatesIDError(t *testing.T) {
	boom := func() (string, error) { return "", fmt.Errorf("no randomness") }
	out, links, err := Assign(`<a href="https://ext.com/z">x</a>`, "https://client.app", boom)
	require.Error(t, err)
	assert.Nil(t, links)
	assert.Equal(t, `<a href="https://ext.com/z">x</a>`, out, "on id error the original content is returned")
}

func TestResolve_SwapsAndExpandsToken(t *testing.T) {
	body := `<a href="id-1">ours</a><a href="id-2">ext</a>`
	byID := map[string]string{
		"id-1": "{CLIENT_URL}/articles/abc",
		"id-2": "https://ext.com/z",
	}
	out := Resolve(body, byID, "https://client.app")
	assert.Contains(t, out, `href="https://client.app/articles/abc"`, "token expanded to the client URL")
	assert.Contains(t, out, `href="https://ext.com/z"`, "external restored verbatim")
}

func TestResolve_EmptyMapReturnsContentUnchanged(t *testing.T) {
	body := `<a href="id-1">x</a>`
	assert.Equal(t, body, Resolve(body, map[string]string{}, "https://client.app"))
}

func TestResolve_UnknownIDLeftUntouched(t *testing.T) {
	// A legacy body with a real URL, or an id missing from the map, passes through.
	body := `<a href="https://legacy.com/a">x</a>`
	out := Resolve(body, map[string]string{"id-1": "https://ext.com/z"}, "https://client.app")
	assert.Equal(t, body, out)
}

func TestResolve_EmptyClientURLYieldsRelativeInternalLink(t *testing.T) {
	out := Resolve(`<a href="id-1">x</a>`, map[string]string{"id-1": "{CLIENT_URL}/articles/abc"}, "")
	assert.Contains(t, out, `href="/articles/abc"`, "empty client URL expands the token to a root-relative link")
}

func TestRoundTrip_AssignThenResolve(t *testing.T) {
	const clientURL = "https://client.app"
	content := `<p>see <a href="https://client.app/articles/abc">ours</a> and <a href="https://ext.com/z">ext</a></p>`

	stored, links, err := Assign(content, clientURL, seqIDs())
	require.NoError(t, err)

	byID := make(map[string]string, len(links))
	for _, l := range links {
		byID[l.ID] = l.Href
	}
	resolved := Resolve(stored, byID, clientURL)

	assert.Contains(t, resolved, `href="https://client.app/articles/abc"`)
	assert.Contains(t, resolved, `href="https://ext.com/z"`)
}

func TestInternalHref(t *testing.T) {
	assert.Equal(t, "{CLIENT_URL}/articles/xyz", InternalHref("xyz"))
}
