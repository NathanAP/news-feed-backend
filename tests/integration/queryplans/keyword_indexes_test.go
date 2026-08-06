// Package queryplans_test guards the query PLANS of the two keyword lookups, not their results.
//
// It exists because this project has been bitten twice by the same class of bug: a GIN index that
// exists, is correct, and is never actually used.
//
//   - 0.37.3: idx_feeds_keywords was unreachable — the layer-1 judgement query expanded the column
//     through jsonb_array_elements_text, a per-row function call no index can serve. Fixed by adding
//     a `?|` predicate (measured 428ms -> 22ms on 60k feeds).
//   - 0.46.4: the `?|` predicates were there, but their right-hand side was
//     `ARRAY(SELECT jsonb_array_elements_text($1::jsonb))` — an InitPlan, opaque at plan time. The
//     planner could not estimate its selectivity, defaulted to a guess, priced the index above a seq
//     scan and chose the scan. The index was usable (enable_seqscan=off proved it); it just was not
//     chosen, and whether it got chosen depended on table size.
//
// Both failures are invisible to a result-based test: the rows returned are correct either way. Only
// the plan tells the truth, so these tests assert on EXPLAIN output.
//
// They insert enough rows that a sequential scan is not trivially cheapest, then ANALYZE so the
// planner has real statistics. Asserting on a plan is inherently a little coupled to the planner, so
// the assertions are deliberately coarse: the index is named in the plan, and articles/feeds are not
// sequentially scanned. Anything finer would break on a Postgres upgrade for no benefit.
package queryplans_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

// rowCount is sized so a seq scan is not free. Small enough to keep the test a few seconds.
const rowCount = 20000

func explainPlan(t *testing.T, database *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := database.Query("EXPLAIN (ANALYZE) "+query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}

// TestQueryPlan_SuggestRelatedKeywords_UsesGinIndex mirrors the WHERE/FROM of SuggestRelatedKeywords.
func TestQueryPlan_SuggestRelatedKeywords_UsesGinIndex(t *testing.T) {
	database := testutils.SetupTestDB(t)

	sourceID := uuid.NewString()
	_, err := database.Exec(
		`INSERT INTO sources (id, status, name, url, url_rss)
		 VALUES ($1, TRUE, 'seed', 'https://seed', 'https://seed/rss')`, sourceID)
	require.NoError(t, err)

	// One article carries the needle; the rest share a handful of common keywords, so the needle is
	// highly selective — the case where choosing the index matters most.
	for i := 0; i < rowCount; i++ {
		keywords := fmt.Sprintf(`["common","bulk%d"]`, i%50)
		if i == 0 {
			keywords = `["metallica","rock"]`
		}
		_, err := database.Exec(
			`INSERT INTO articles (id, status, title, content, url_original, keywords, source_id)
			 VALUES ($1, TRUE, 't', 'c', $2, $3::jsonb, $4)`,
			uuid.NewString(), fmt.Sprintf("https://example.com/%d", i), keywords, sourceID)
		require.NoError(t, err)
	}
	_, err = database.Exec("ANALYZE articles")
	require.NoError(t, err)

	plan := explainPlan(t, database, `
SELECT kw.value::text AS keyword, COUNT(*) AS occurrences
FROM articles a
CROSS JOIN LATERAL jsonb_array_elements_text(a.keywords) AS kw(value)
WHERE a.status = TRUE AND a.removed_at IS NULL
  AND a.keywords ?| $1::text[]
  AND kw.value <> ALL($1::text[])
GROUP BY kw.value
ORDER BY occurrences DESC, keyword ASC
LIMIT $2`, "{metallica}", 10)

	assert.Contains(t, plan, "idx_articles_keywords",
		"the `?|` predicate must reach the GIN index; if this fails, check that its right-hand side is "+
			"still a plain text[] and not a jsonb subquery.\nPlan:\n"+plan)
	assert.NotContains(t, plan, "Seq Scan on articles",
		"the planner fell back to a sequential scan.\nPlan:\n"+plan)
}

// TestQueryPlan_FindCandidateFeedsByKeywords_UsesGinIndex mirrors the WHERE/FROM of the layer-1
// judgement query. This is the hot one: it runs per discovered article on every CRON sweep, and it is
// the filter that decides how much AI gets paid for.
func TestQueryPlan_FindCandidateFeedsByKeywords_UsesGinIndex(t *testing.T) {
	database := testutils.SetupTestDB(t)

	userID := uuid.NewString()
	_, err := database.Exec(
		`INSERT INTO users (id, google_id, email, name) VALUES ($1, $2, $3, 'seed')`,
		userID, uuid.NewString(), uuid.NewString()+"@example.com")
	require.NoError(t, err)

	for i := 0; i < rowCount; i++ {
		keywords := fmt.Sprintf(`["common","bulk%d"]`, i%50)
		if i == 0 {
			keywords = `["metallica","rock"]`
		}
		_, err := database.Exec(
			`INSERT INTO feeds (id, status, name, keywords, user_id)
			 VALUES ($1, TRUE, 'f', $2::jsonb, $3)`,
			uuid.NewString(), keywords, userID)
		require.NoError(t, err)
	}
	_, err = database.Exec("ANALYZE feeds")
	require.NoError(t, err)

	plan := explainPlan(t, database, `
SELECT f.id, COUNT(DISTINCT fk.value) AS overlap_count
FROM feeds f
JOIN users u ON u.id = f.user_id
  AND u.status = TRUE AND u.removed_at IS NULL
  AND ($2::int = -1
       OR u.last_active_at > CURRENT_TIMESTAMP - make_interval(days => $2::int))
CROSS JOIN LATERAL jsonb_array_elements_text(f.keywords) AS fk(value)
JOIN unnest($1::text[]) AS ak(value) ON ak.value = fk.value
WHERE f.status = TRUE AND f.removed_at IS NULL
  AND f.keywords ?| $1::text[]
GROUP BY f.id`, "{metallica}", -1)

	assert.Contains(t, plan, "idx_feeds_keywords",
		"layer-1 judgement must reach the GIN index; if this fails the CRON is scanning every feed of "+
			"every user per article, and the AI bill follows.\nPlan:\n"+plan)
	assert.NotContains(t, plan, "Seq Scan on feeds",
		"the planner fell back to a sequential scan.\nPlan:\n"+plan)
}
