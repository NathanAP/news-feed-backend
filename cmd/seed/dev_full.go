package main

// runDevFull runs every seed command in dependency order (user → sources → feeds → articles →
// articles-feeds), then issues a dev login so the operator ends with a ready-to-use token. Each
// step prints its own report; the run stops at the first error.
func runDevFull(sc *seedCtx) error {
	steps := []struct {
		name string
		run  func(*seedCtx) (*report, error)
	}{
		{"user", runDevUser},
		{"sources", runDevSources},
		{"feeds", runDevFeeds},
		{"articles", runDevArticles},
		{"articles-feeds", runDevArticleFeeds},
		{"login", runDevLogin},
	}

	for _, step := range steps {
		rep, err := step.run(sc)
		if err != nil {
			return err
		}
		rep.print(step.name)
	}
	return nil
}
