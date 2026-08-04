// Command seed populates the development database with example data (a dev user, sources, articles,
// feeds and their associations) without depending on the discovery CRON. It is a single binary with
// a subcommand — the Taskfile in this folder wraps each subcommand as a `task` target. Every command
// refuses to run outside ENVIRONMENT=development. See instructions.md.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/nathanap/news-feed-backend/services/controllers"
)

// seedCtx bundles everything the seed commands need: the transaction runner, the parsed examples and
// the controllers (reused from the app so the seed follows the exact same write rules).
type seedCtx struct {
	ctx          context.Context
	runTx        controllers.TransactionRunner
	ex           examples
	userCtrl     *controllers.UserController
	prefCtrl     *controllers.UserPreferencesController
	refreshCtrl  *controllers.RefreshTokenController
	sourceCtrl   *controllers.SourceController
	articleCtrl  *controllers.ArticleController
	outboundCtrl *controllers.ArticleOutboundLinkController
	feedCtrl     *controllers.FeedController
	afCtrl       *controllers.ArticleFeedController
	authCtrl     *controllers.AuthController
}

// commands maps subcommand name → handler. `full` runs them all in dependency order.
var commands = map[string]func(*seedCtx) (*report, error){
	"user":           runDevUser,
	"login":          runDevLogin,
	"sources":        runDevSources,
	"articles":       runDevArticles,
	"feeds":          runDevFeeds,
	"articles-feeds": runDevArticleFeeds,
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: seed <%s|full>", availableCommands())
	}
	command := os.Args[1]

	if err := requireDevEnvironment(); err != nil {
		log.Fatalf("seed aborted: %v", err)
	}

	root, err := projectRoot()
	if err != nil {
		log.Fatalf("seed aborted: %v", err)
	}
	ex, err := loadExamples(root)
	if err != nil {
		log.Fatalf("seed aborted: %v", err)
	}

	database, err := openDB()
	if err != nil {
		log.Fatalf("seed aborted: %v", err)
	}
	defer database.Close()

	sc := buildSeedContext(database, ex)

	if command == "full" {
		if err := runDevFull(sc); err != nil {
			log.Fatalf("seed full failed: %v", err)
		}
		return
	}

	handler, ok := commands[command]
	if !ok {
		log.Fatalf("unknown command %q (available: %s, full)", command, availableCommands())
	}

	rep, err := handler(sc)
	if err != nil {
		log.Fatalf("seed %s failed: %v", command, err)
	}
	rep.print(command)
}

// buildSeedContext wires the controllers exactly as the API does, so the seed writes go through the
// same rules. The AuthController only needs its JWT config here (used by dev-login); OAuth is nil.
func buildSeedContext(database *sql.DB, ex examples) *seedCtx {
	runTx := controllers.NewTransactionRunner(database)
	userCtrl := controllers.NewUserController()
	prefCtrl := controllers.NewUserPreferencesController()
	refreshCtrl := controllers.NewRefreshTokenController(parseRefreshTokenExpiry())
	authCtrl := controllers.NewAuthController(
		nil, userCtrl, refreshCtrl, prefCtrl, runTx,
		[]byte(os.Getenv("JWT_SECRET_KEY")), parseAccessTokenExpiry(),
	)

	return &seedCtx{
		ctx:          context.Background(),
		runTx:        runTx,
		ex:           ex,
		userCtrl:     userCtrl,
		prefCtrl:     prefCtrl,
		refreshCtrl:  refreshCtrl,
		sourceCtrl:   controllers.NewSourceController(),
		articleCtrl:  controllers.NewArticleController(),
		outboundCtrl: controllers.NewArticleOutboundLinkController(),
		feedCtrl:     controllers.NewFeedController(),
		afCtrl:       controllers.NewArticleFeedController(),
		authCtrl:     authCtrl,
	}
}

func availableCommands() string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Sprintf("%v", names)
}

// parseAccessTokenExpiry / parseRefreshTokenExpiry read the same env vars the API uses.
func parseAccessTokenExpiry() time.Duration {
	minutes, _ := strconv.Atoi(os.Getenv("JWT_ACCESS_TOKEN_EXPIRY_MINUTES"))
	if minutes <= 0 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

func parseRefreshTokenExpiry() time.Duration {
	days, _ := strconv.Atoi(os.Getenv("JWT_REFRESH_TOKEN_EXPIRY_DAYS"))
	if days <= 0 {
		days = 30
	}
	return time.Duration(days) * 24 * time.Hour
}
