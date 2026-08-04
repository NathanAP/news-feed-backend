package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// runDevArticles creates the example articles. Unlike the other seeds it is intentionally NOT
// idempotent: every run creates the articles again, each with a fresh random url_original (so the
// unique constraint is never hit). Articles are linked round-robin to the sources that exist in the
// DB, so `sources` must be seeded first.
func runDevArticles(sc *seedCtx) (*report, error) {
	// Mirror the discovery pipeline's 0.45 storage format: bodies are stored with outbound-link ids in
	// their anchors, not URLs. The example links are all external, so they are stored literally in the
	// rows (no CLIENT_URL token). Without this, seeded articles would keep raw URLs and not exercise
	// the read-time swap in dev.
	clientURL := strings.TrimRight(os.Getenv("CLIENT_URL"), "/")

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

			saved, err := sc.articleCtrl.Create(sc.ctx, q, a.Title, a.Content, url, source.ID, a.Keywords, language)
			if err != nil {
				return err
			}

			// Assign outbound-link ids to the body's anchors and persist the links + id-form body.
			rewritten, links, aerr := outboundlinks.Assign(saved.Content, clientURL, newSeedOutboundID)
			if aerr != nil {
				return aerr
			}
			for _, l := range links {
				if _, err := sc.outboundCtrl.Create(sc.ctx, q, l.ID, saved.ID, l.Href); err != nil {
					return err
				}
			}
			if len(links) > 0 {
				if err := sc.articleCtrl.UpdateContent(sc.ctx, q, saved.ID, rewritten); err != nil {
					return err
				}
			}
			rep.add(fmt.Sprintf("article %q -> source %s (%d outbound links)", truncate(a.Title, 50), source.Url, len(links)))
		}
		return nil
	})
	return rep, err
}

// newSeedOutboundID matches the outboundlinks.Assign newID contract with a UUIDv7.
func newSeedOutboundID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
