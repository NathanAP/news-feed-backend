package langdetect_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/langdetect"
)

func requireNotProduction(t *testing.T) {
	t.Helper()
	env := os.Getenv("ENVIRONMENT")
	require.NotEqual(t, "production", env, "tests must not run in production")
	require.NotEqual(t, "staging", env, "tests must not run in staging")
}

func TestDetect_KnownLanguages(t *testing.T) {
	requireNotProduction(t)

	d := langdetect.New()

	cases := map[string]struct {
		title, content, want string
	}{
		"portuguese": {"Rock in Rio vende ingressos", "<p>O festival acontece em setembro no Parque Olímpico.</p>", "pt"},
		"english":    {"Band announces new world tour", "<p>The concert dates were revealed today.</p>", "en"},
		"spanish":    {"La banda anuncia nueva gira mundial", "<p>Las fechas del concierto se revelaron hoy.</p>", "es"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			code, ok := d.Detect(tc.title, tc.content)
			require.True(t, ok)
			assert.Equal(t, tc.want, code)
		})
	}
}

func TestDetect_StripsHTMLBeforeDetecting(t *testing.T) {
	requireNotProduction(t)

	d := langdetect.New()
	// Heavy HTML around a clearly Portuguese sentence must not skew detection to English.
	code, ok := d.Detect(
		"Notícia importante",
		`<div class="post"><p><strong>A</strong> seleção brasileira venceu a partida decisiva ontem à noite.</p></div>`,
	)
	require.True(t, ok)
	assert.Equal(t, "pt", code)
}

func TestDetect_EmptyReturnsNotOk(t *testing.T) {
	requireNotProduction(t)

	d := langdetect.New()
	_, ok := d.Detect("", "   ")
	assert.False(t, ok)
}
