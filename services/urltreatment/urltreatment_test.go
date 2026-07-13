package urltreatment_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/urltreatment"
)

// fakeResolve returns internal URLs from a fixed map, recording every batch of hrefs it was asked about.
func fakeResolve(mapping map[string]string, calls *[][]string) urltreatment.Resolve {
	return func(_ context.Context, hrefs []string) (map[string]string, error) {
		if calls != nil {
			*calls = append(*calls, hrefs)
		}
		out := make(map[string]string)
		for _, h := range hrefs {
			if v, ok := mapping[h]; ok {
				out[h] = v
			}
		}
		return out, nil
	}
}

func TestTreat_RewritesInternalLeavesExternal(t *testing.T) {
	content := `<p>see <a href="https://src.com/known">ours</a> and <a href="https://ext.com/x">ext</a></p>`
	resolve := fakeResolve(map[string]string{"https://src.com/known": "https://client.app/articles/abc"}, nil)

	out, err := urltreatment.Treat(context.Background(), content, resolve, false)
	require.NoError(t, err)
	assert.Contains(t, out, `href="https://client.app/articles/abc"`, "internal link rewritten")
	assert.Contains(t, out, `href="https://ext.com/x"`, "external link untouched")
	assert.Contains(t, out, "ours")
	assert.Contains(t, out, "ext")
}

func TestTreat_NoAnchorsReturnsOriginal(t *testing.T) {
	content := `<p>just <strong>text</strong>, no links</p>`
	out, err := urltreatment.Treat(context.Background(), content, fakeResolve(nil, nil), false)
	require.NoError(t, err)
	assert.Equal(t, content, out)
}

func TestTreat_NoMatchesReturnsOriginal(t *testing.T) {
	content := `<p><a href="https://ext.com/a">a</a></p>`
	out, err := urltreatment.Treat(context.Background(), content, fakeResolve(nil, nil), false)
	require.NoError(t, err)
	assert.Equal(t, content, out) // nothing to rewrite → original preserved verbatim
}

func TestTreat_ResolvesEachUniqueHrefOnce(t *testing.T) {
	content := `<a href="https://src.com/a">1</a><a href="https://src.com/a">2</a><a href="https://src.com/b">3</a>`
	var calls [][]string
	resolve := fakeResolve(map[string]string{
		"https://src.com/a": "https://client.app/articles/a",
		"https://src.com/b": "https://client.app/articles/b",
	}, &calls)

	out, err := urltreatment.Treat(context.Background(), content, resolve, false)
	require.NoError(t, err)
	require.Len(t, calls, 1, "resolve is called exactly once for the whole batch")
	assert.ElementsMatch(t, []string{"https://src.com/a", "https://src.com/b"}, calls[0], "only unique hrefs are resolved")
	assert.Equal(t, 2, strings.Count(out, `href="https://client.app/articles/a"`), "both anchors sharing a href are rewritten")
	assert.Contains(t, out, `href="https://client.app/articles/b"`)
}

func TestTreat_ResolveErrorReturnsOriginal(t *testing.T) {
	content := `<p><a href="https://src.com/known">x</a></p>`
	boom := errors.New("db down")
	resolve := func(_ context.Context, _ []string) (map[string]string, error) {
		return nil, boom
	}
	out, err := urltreatment.Treat(context.Background(), content, resolve, false)
	require.ErrorIs(t, err, boom)
	assert.Equal(t, content, out) // best-effort: original content on error
}
