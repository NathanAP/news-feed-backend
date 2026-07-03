package main

import (
	"fmt"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// runDevArticleFeeds links every active article to every feed of the dev user, bypassing judgement
// (that is the whole point in dev). Idempotent: an article already linked to a feed is skipped, so
// re-running does not create duplicate junction rows.
func runDevArticleFeeds(sc *seedCtx) (*report, error) {
	rep := &report{}
	err := sc.runTx(sc.ctx, func(q db.Querier) error {
		user, err := requireDevUser(sc, q)
		if err != nil {
			return err
		}

		feeds, err := sc.feedCtrl.List(sc.ctx, q, user.ID)
		if err != nil {
			return err
		}
		if len(feeds) == 0 {
			return fmt.Errorf("no feeds for the dev user — run `task sdf` (seed-dev-feeds) first")
		}

		articles, err := sc.articleCtrl.List(sc.ctx, q)
		if err != nil {
			return err
		}
		if len(articles) == 0 {
			return fmt.Errorf("no articles found — run `task sda` (seed-dev-articles) first")
		}

		for _, article := range articles {
			// Existing links for this article (from the dev user's perspective) → skip set.
			existing, err := sc.afCtrl.FindByArticleAndUser(sc.ctx, q, article.ID, user.ID)
			if err != nil {
				return err
			}
			linked := make(map[string]struct{}, len(existing))
			for _, af := range existing {
				linked[af.FeedID] = struct{}{}
			}

			for _, feed := range feeds {
				if _, ok := linked[feed.ID]; ok {
					rep.skip(fmt.Sprintf("%q <-> feed %q (already linked)", truncate(article.Title, 40), feed.Name))
					continue
				}
				if _, err := sc.afCtrl.Create(sc.ctx, q, article.ID, feed.ID); err != nil {
					return err
				}
				rep.add(fmt.Sprintf("%q <-> feed %q", truncate(article.Title, 40), feed.Name))
			}
		}
		return nil
	})
	return rep, err
}
