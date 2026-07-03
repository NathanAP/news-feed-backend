package main

import (
	"strings"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// runDevFeeds creates the example feeds for the dev user. Idempotent: a feed whose name already
// exists for the user is skipped. Goes through FeedController.Create, so the per-user limit and
// keyword rules are enforced (the examples must respect them).
func runDevFeeds(sc *seedCtx) (*report, error) {
	rep := &report{}
	err := sc.runTx(sc.ctx, func(q db.Querier) error {
		user, err := requireDevUser(sc, q)
		if err != nil {
			return err
		}

		existing, err := sc.feedCtrl.List(sc.ctx, q, user.ID)
		if err != nil {
			return err
		}
		known := make(map[string]struct{}, len(existing))
		for _, f := range existing {
			known[strings.ToLower(f.Name)] = struct{}{}
		}

		for _, f := range sc.ex.Feeds {
			if _, ok := known[strings.ToLower(f.Name)]; ok {
				rep.skip("feed " + f.Name + " (already exists)")
				continue
			}
			if _, err := sc.feedCtrl.Create(sc.ctx, q, user.ID, f.Name, f.Keywords); err != nil {
				return err
			}
			rep.add("feed " + f.Name)
		}
		return nil
	})
	return rep, err
}
