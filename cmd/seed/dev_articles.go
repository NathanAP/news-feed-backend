package main

import (
	"fmt"

	"github.com/google/uuid"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// runDevArticles creates the example articles. Unlike the other seeds it is intentionally NOT
// idempotent: every run creates the articles again, each with a fresh random url_original (so the
// unique constraint is never hit). Articles are linked round-robin to the sources that exist in the
// DB, so `sources` must be seeded first.
func runDevArticles(sc *seedCtx) (*report, error) {
	rep := &report{}
	err := sc.runTx(sc.ctx, func(q db.Querier) error {
		sources, err := sc.sourceCtrl.ListAll(sc.ctx, q)
		if err != nil {
			return err
		}
		if len(sources) == 0 {
			return fmt.Errorf("no active sources found — run `task sds` (seed-dev-sources) first")
		}

		for i, a := range sc.ex.Articles {
			url := fmt.Sprintf("https://www.article-%s.com.br/feed/", uuid.NewString())
			source := sources[i%len(sources)]

			var language *string
			if a.LanguageOriginal != "" {
				lang := a.LanguageOriginal
				language = &lang
			}

			if _, err := sc.articleCtrl.Create(sc.ctx, q, a.Title, a.Content, url, source.ID, a.Keywords, language); err != nil {
				return err
			}
			rep.add(fmt.Sprintf("article %q -> source %s", truncate(a.Title, 50), source.Url))
		}
		return nil
	})
	return rep, err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
