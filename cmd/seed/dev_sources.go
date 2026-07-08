package main

import (
	"strings"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// runDevSources creates the example sources. Idempotent: a source whose url already exists (active)
// is skipped, so re-running does not hit the unique constraint.
func runDevSources(sc *seedCtx) (*report, error) {
	rep := &report{}
	err := sc.runTx(sc.ctx, func(q db.Querier) error {
		existing, err := sc.sourceCtrl.List(sc.ctx, q)
		if err != nil {
			return err
		}
		known := make(map[string]struct{}, len(existing))
		for _, s := range existing {
			known[strings.ToLower(s.Url)] = struct{}{}
		}

		for _, s := range sc.ex.Sources {
			if _, ok := known[strings.ToLower(s.URL)]; ok {
				rep.skip("source " + s.URL + " (already exists)")
				continue
			}
			if _, err := sc.sourceCtrl.Create(sc.ctx, q, s.Name, s.URL, s.URLRss); err != nil {
				return err
			}
			rep.add("source " + s.URL)
		}
		return nil
	})
	return rep, err
}
