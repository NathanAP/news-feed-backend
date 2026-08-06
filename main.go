package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/nathanap/news-feed-backend/logger"
	"github.com/nathanap/news-feed-backend/middlewares"
	"github.com/nathanap/news-feed-backend/services/ai"
	"github.com/nathanap/news-feed-backend/services/ai/gemini"
	"github.com/nathanap/news-feed-backend/services/ai/openaicompat"
	"github.com/nathanap/news-feed-backend/services/controllers"
	"github.com/nathanap/news-feed-backend/services/cron"
	"github.com/nathanap/news-feed-backend/services/discovery"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	feedendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/feeds"
	healthendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/health"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
	systemendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/system"
	userendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/users"
	"github.com/nathanap/news-feed-backend/services/judgement"
	"github.com/nathanap/news-feed-backend/services/langdetect"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Bootstrap logging goes through the standard logger, which stamps lines in the host's timezone by
	// default. Everything else in this application is UTC (conventions.md), so a log line saying 16:08
	// next to a response saying 19:08Z is a trap for whoever is reading both at once.
	log.SetFlags(log.LstdFlags | log.LUTC)

	logger.Setup()

	// The whole connection is carried by DATABASE_URL (a libpq DSN, e.g.
	// postgres://user:pass@host:5432/dbname?sslmode=disable). A single URL is what managed providers
	// hand out, so pointing the API at a hosted database is a config change, never a code change.
	// The driver is pgx registered under the name "pgx" by its stdlib shim: pgx does the talking,
	// while the app keeps the driver-agnostic database/sql surface the controllers are written against.
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatalf("DATABASE_URL must be set")
	}

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := database.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully")

	if err := runMigrations(database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	accessTokenExpiry, err := parseMinutes(os.Getenv("JWT_ACCESS_TOKEN_EXPIRY_MINUTES"))
	if err != nil {
		log.Fatalf("Invalid JWT_ACCESS_TOKEN_EXPIRY_MINUTES: %v", err)
	}

	refreshTokenExpiry, err := parseDays(os.Getenv("JWT_REFRESH_TOKEN_EXPIRY_DAYS"))
	if err != nil {
		log.Fatalf("Invalid JWT_REFRESH_TOKEN_EXPIRY_DAYS: %v", err)
	}

	jwtSecretValue := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretValue == "" {
		log.Fatalf("JWT_SECRET_KEY must be set")
	}
	jwtSecret := []byte(jwtSecretValue)

	oauth2Config := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	runTx := controllers.NewTransactionRunner(database)
	prefCtrl := controllers.NewUserPreferencesController()
	userCtrl := controllers.NewUserController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(refreshTokenExpiry)
	authCtrl := controllers.NewAuthController(oauth2Config, userCtrl, refreshTokenCtrl, prefCtrl, runTx, jwtSecret, accessTokenExpiry)
	sourceCtrl := controllers.NewSourceController()
	articleCtrl := controllers.NewArticleController()
	outboundCtrl := controllers.NewArticleOutboundLinkController()
	afCtrl := controllers.NewArticleFeedController()
	feedCtrl := controllers.NewFeedController()
	systemCtrl := controllers.NewSystemController()

	// Article-body treatment is deterministic (no AI): url treatment rewrites in-content links to
	// articles we already have (to CLIENT_URL/articles/{id}), then the bluemonday whitelist in
	// services/sanitize cleans the raw RSS HTML. Both are wired directly where used (discovery
	// pipeline and dry-run endpoint). Without CLIENT_URL, url treatment is skipped (links only sanitized).
	// clientURL also feeds the read-time outbound-link swap (0.45): stored internal hrefs hold the
	// {CLIENT_URL} token, expanded back to this value when an article is served.
	clientURL := strings.TrimRight(os.Getenv("CLIENT_URL"), "/")
	if clientURL == "" {
		log.Println("CLIENT_URL not set; url treatment (internal article links) will be skipped")
	}
	urlTreatmentVerbose := os.Getenv("URLS_TREATMENT_VERBOSE_MODE") == "true"

	keyworders, keywordsDefaultMode := buildKeyworders()
	defaultKeyworder := keyworders[keywordsDefaultMode]
	if defaultKeyworder == nil {
		log.Printf("KEYWORDS_MODE %q not recognized (use local, groq or gemini); keywords disabled by default", keywordsDefaultMode)
		defaultKeyworder = ai.NewDisabledClient()
	}

	judgers, judgementDefaultMode := buildJudgers()
	judgementThreshold := parseThreshold(os.Getenv("JUDGEMENT_THRESHOLD"))
	judgementAutoAssociateRatio := parseAutoAssociateRatio(os.Getenv("JUDGEMENT_AUTOASSOCIATE_RATIO"))
	judgementMinMatches := parseMinMatches(os.Getenv("JUDGEMENT_MIN_MATCHES"))
	// How many days a user can go without any activity (token refresh or login) before the discovery
	// filter stops routing news to their feeds. -1 disables the filter (every user counts as active).
	inactiveDays := parseInactiveDays(os.Getenv("DAYS_UNTIL_USER_IS_INACTIVE"))
	defaultJudger := judgers[judgementDefaultMode]
	if defaultJudger == nil {
		log.Printf("JUDGEMENT_MODE %q not recognized (use local, groq or gemini); judgement disabled by default", judgementDefaultMode)
		defaultJudger = ai.NewDisabledClient()
	}
	evaluator := judgement.NewEvaluator(defaultJudger, judgementThreshold, judgementAutoAssociateRatio, judgementMinMatches)

	// Language detection (lingua-go) is used by the treatment step; not AI, so no provider/mode.
	detector := langdetect.New()

	// Translation is LLM-only (quality-sensitive user-facing text): a single provider via TRANSLATION_*.
	translationProvider, translationModel := os.Getenv("TRANSLATION_PROVIDER"), os.Getenv("TRANSLATION_MODEL")
	var translator ai.Translator = buildProvider(translationProvider, translationModel)
	translator = ai.NewVerboseTranslator(translator, fmt.Sprintf("%s/%s", translationProvider, translationModel), os.Getenv("TRANSLATION_VERBOSE_MODE") == "true")

	authMiddleware := middlewares.NewAuthMiddleware(jwtSecret, refreshTokenCtrl, runTx)

	// Administrator authorization always reads users.admin from the database, never the token claim
	// (see middlewares/admin.go). The same resolver backs both the admin-only routes and the
	// maintenance bypass, so there is one definition of "is an administrator" in the whole API.
	adminResolver := middlewares.NewAdminResolver(jwtSecret, refreshTokenCtrl, userCtrl, runTx)
	requireAdmin := middlewares.NewRequireAdminMiddleware(adminResolver)

	app := fiber.New(fiber.Config{
		AppName: os.Getenv("PROJECT_NAME"),
	})

	// Recover is mounted first so it wraps every middleware and handler below. Fiber does NOT
	// install it by default: without it a panic anywhere in a request unwinds past Fiber and takes
	// the whole process down, which conventions.md forbids ("exceções não devem derrubar a
	// aplicação") - one malformed request would stop serving every other user. With it, the panic
	// becomes a 500 for that request alone and the API stays up.
	app.Use(recover.New())

	// CORS is mounted right after, before every route and guard, so it also covers the preflight
	// OPTIONS and the maintenance/health responses. The web client lives on a different origin, so
	// without this the browser blocks all cross-origin calls.
	app.Use(middlewares.NewCORSMiddleware())

	apiVersion := os.Getenv("API_VERSION")
	api := app.Group("/" + apiVersion)

	oauthRedirectAllowlist := parseCSV(os.Getenv("OAUTH_ALLOWED_REDIRECT_URIS"))

	// --- Routes exempt from the maintenance guard, registered before it is mounted ---
	//
	// /health so monitoring can still see the application; the app-status toggle so the API can always
	// be brought back online through the API itself; and the entire /auth group so authentication
	// survives a maintenance window.
	//
	// That last exemption is not convenience, it is a deadlock fix: the maintenance bypass identifies
	// an administrator from their access token, but access tokens expire in
	// JWT_ACCESS_TOKEN_EXPIRY_MINUTES and the refresh token carries no identity the bypass can read.
	// With /auth behind the guard, an administrator whose token expired mid-maintenance gets 503 from
	// /auth/refresh — the one call that would let them reach the toggle that ends the maintenance —
	// and the only way back in is editing the database by hand. A regular user can still obtain a
	// token while the application is down; every route that actually does something answers them 503.
	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))
	api.Put("/system/app-status", adminRoute(authMiddleware, requireAdmin, systemendpoints.UpdateAppStatus(systemCtrl, runTx))...)

	auth := api.Group("/auth")
	auth.Get("/google", authendpoints.GoogleLogin(oauth2Config, jwtSecret, oauthRedirectAllowlist))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, jwtSecret))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl, runTx))...)
	auth.Delete("/invalidate", adminRoute(authMiddleware, requireAdmin, authendpoints.Invalidate(refreshTokenCtrl, runTx))...)
	auth.Delete("/invalidate-all", adminRoute(authMiddleware, requireAdmin, authendpoints.InvalidateAll(refreshTokenCtrl, runTx))...)

	// Global maintenance guard: every route registered below returns 503 while app_status is off,
	// except for requests coming from an administrator. The routes above are registered earlier and
	// therefore never reach it at all.
	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx, adminResolver))

	users := api.Group("/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)
	users.Get("/me/preferences", append(authMiddleware, userendpoints.GetPreferences())...)
	users.Put("/me/preferences", append(authMiddleware, userendpoints.UpdatePreferences(prefCtrl, authCtrl, runTx, int(accessTokenExpiry.Seconds())))...)

	// Development-only: log in the seeded dev user without Google OAuth. Registered conditionally so
	// the route literally does not exist outside development (defense in depth beyond any runtime check).
	if os.Getenv("ENVIRONMENT") == "development" {
		users.Post("/dev-login", userendpoints.DevLogin(userCtrl, prefCtrl, refreshTokenCtrl, authCtrl, runTx, int(accessTokenExpiry.Seconds())))
		log.Println("Development mode: POST /v1/users/dev-login enabled")
	}

	discoveryHTTPClient := &http.Client{Timeout: 30 * time.Second}

	// Sources are public to read (PROJECT.md: every user sees the same predefined set) and
	// administrator-only to change, which is why the guard is per route rather than on the group.
	sources := api.Group("/sources")
	sources.Post("/create", adminRoute(authMiddleware, requireAdmin, sourceendpoints.CreateSource(sourceCtrl, runTx))...)
	sources.Get("/rss-discovery", append(authMiddleware, sourceendpoints.RSSDiscovery(&http.Client{}))...)
	sources.Get("/:id/article-discovery", adminRoute(authMiddleware, requireAdmin, sourceendpoints.SourceArticleDiscovery(sourceCtrl, articleCtrl, runTx, discoveryHTTPClient))...)
	sources.Get("/:id", append(authMiddleware, sourceendpoints.GetSource(sourceCtrl, runTx))...)
	sources.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)
	sources.Put("/:id", adminRoute(authMiddleware, requireAdmin, sourceendpoints.UpdateSource(sourceCtrl, runTx))...)
	sources.Delete("/:id", adminRoute(authMiddleware, requireAdmin, sourceendpoints.DeleteSource(sourceCtrl, runTx))...)

	// Articles are readable by every user; writing them is an administrator escape hatch for a news
	// item that got out of hand (PROJECT.md), not part of the normal flow — the pipeline is what
	// creates articles.
	articles := api.Group("/articles")
	articles.Post("/create", adminRoute(authMiddleware, requireAdmin, articleendpoints.CreateArticle(articleCtrl, runTx))...)
	articles.Get("/:id/translate", append(authMiddleware, articleendpoints.TranslateArticle(articleCtrl, outboundCtrl, translator, runTx, clientURL))...)
	articles.Put("/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(articleCtrl, afCtrl, runTx))...)
	articles.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, afCtrl, outboundCtrl, runTx, clientURL))...)
	articles.Get("", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, outboundCtrl, runTx, clientURL))...)
	articles.Put("/:id", adminRoute(authMiddleware, requireAdmin, articleendpoints.UpdateArticle(articleCtrl, runTx))...)
	articles.Delete("/:id", adminRoute(authMiddleware, requireAdmin, articleendpoints.DeleteArticle(articleCtrl, outboundCtrl, runTx))...)

	// Development-only dry-run tools for the AI pipeline (treatment, judgement). They call the AI
	// for real (consume quota) and expose internal pipeline behavior, so they must never be reachable
	// by clients in staging/production. Registered conditionally, like /users/dev-login, so the routes
	// literally do not exist outside development (defense in depth beyond any runtime check).
	if os.Getenv("ENVIRONMENT") == "development" {
		articles.Post("/treatment", append(authMiddleware, articleendpoints.TreatArticle(keyworders, keywordsDefaultMode, detector, articleCtrl, runTx, clientURL, urlTreatmentVerbose))...)
		articles.Post("/judgement", append(authMiddleware, articleendpoints.JudgeArticle(feedCtrl, judgers, judgementDefaultMode, judgementThreshold, judgementAutoAssociateRatio, judgementMinMatches, inactiveDays, runTx))...)
		log.Println("Development mode: POST /v1/articles/treatment and /v1/articles/judgement enabled")
	}

	feeds := api.Group("/feeds")
	feeds.Post("/create", append(authMiddleware, feedendpoints.CreateFeed(feedCtrl, runTx))...)
	// Static route registered before "/:id" so "check-for-new-articles" is never swallowed as an id
	// (Fiber prioritizes static over param, but keeping the order explicit matches sources/rss-discovery).
	feeds.Get("/check-for-new-articles", append(authMiddleware, feedendpoints.CheckForNewArticles(afCtrl, runTx))...)
	// Static route registered before "/:id" so it is never swallowed as an id. Draws from the global
	// article pool, so it takes articleCtrl rather than feedCtrl.
	keywordSuggestionsWindowDays := parseSuggestionWindowDays(os.Getenv("KEYWORD_SUGGESTIONS_WINDOW_DAYS"))
	feeds.Get("/keyword-suggestions", append(authMiddleware, feedendpoints.SuggestKeywords(articleCtrl, runTx, keywordSuggestionsWindowDays))...)
	feeds.Get("/:id/articles", append(authMiddleware, feedendpoints.FeedArticles(feedCtrl, afCtrl, outboundCtrl, runTx, clientURL))...)
	feeds.Get("/:id", append(authMiddleware, feedendpoints.GetFeed(feedCtrl, runTx))...)
	feeds.Get("", append(authMiddleware, feedendpoints.ListFeeds(feedCtrl, runTx))...)
	feeds.Put("/:id", append(authMiddleware, feedendpoints.UpdateFeed(feedCtrl, runTx))...)
	feeds.Delete("/:id", append(authMiddleware, feedendpoints.DeleteFeed(feedCtrl, runTx))...)

	if os.Getenv("RSS_FEED_CRON_ACTIVE") == "true" {
		cronVerbose := os.Getenv("RSS_FEED_CRON_VERBOSE_MODE") == "true"
		discoveryConcurrency := parseConcurrency(os.Getenv("DISCOVERY_CONCURRENCY"))
		discoveryMaxArticles := parseMaxArticles(os.Getenv("DISCOVERY_MAX_ARTICLES"))
		processor := discovery.NewTreatmentProcessor(runTx, articleCtrl, outboundCtrl, feedCtrl, afCtrl, detector, defaultKeyworder, evaluator, clientURL, urlTreatmentVerbose, discoveryConcurrency, inactiveDays, cronVerbose)
		runner := cron.NewDiscoveryRunner(runTx, sourceCtrl, systemCtrl, processor, discoveryHTTPClient, discoveryConcurrency, discoveryMaxArticles, cronVerbose)
		scheduler, err := cron.NewScheduler(os.Getenv("RSS_FEED_CRON_SCHEDULE"), runner)
		if err != nil {
			log.Fatalf("Failed to set up discovery cron: %v", err)
		}
		scheduler.Start()
		log.Println("Discovery cron started")
	}

	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "3000"
	}

	log.Fatal(app.Listen(":" + apiPort))
}

// adminRoute composes the handler chain of an administrator-only route: authenticate, then authorize
// against the database, then run the handler.
//
// It copies the shared auth chain into a fresh slice on purpose. Writing
// `adminChain := append(authMiddleware, requireAdmin)` once and appending a handler to it per route
// would give every route the same backing array, and each registration would overwrite the previous
// route's handler in place — the routes would silently end up pointing at whichever handler was
// registered last.
func adminRoute(authMiddleware []fiber.Handler, requireAdmin, handler fiber.Handler) []fiber.Handler {
	chain := make([]fiber.Handler, 0, len(authMiddleware)+2)
	chain = append(chain, authMiddleware...)
	return append(chain, requireAdmin, handler)
}

// SLM/LLM models are fixed to the ones declared in the stack (CLAUDE.md); the Groq model is
// configurable because the stack does not pin a specific one.
const (
	localModel                  = "qwen3:4b"
	geminiModel                 = "gemini-2.5-flash"
	defaultGroqBaseURL          = "https://api.groq.com/openai/v1"
	defaultOllamaURL            = "http://localhost:11434"
	defaultJudgementThreshold   = 70
	defaultDiscoveryConcurrency = 1
	// Layer-2 triage defaults: a feed whose keywords are >= 30% covered by the article auto-associates
	// (no AI); a feed with fewer than 2 overlapping keywords (i.e. just 1) is discarded (no AI).
	defaultJudgementAutoAssociateRatio = 0.30
	defaultJudgementMinMatches         = 2
	// "popular" keyword suggestions look back this many days by default, so they track what is hot
	// now rather than all-time. -1 in the env disables the window.
	defaultKeywordSuggestionsWindowDays = 30
	// A user not seen (token refresh or login) within this many days is skipped by the discovery
	// filter, so the CRON stops routing news to abandoned accounts. -1 disables the filter.
	defaultDaysUntilUserIsInactive = 15
)

// buildModeClients pre-builds an AI client for every mode (local | groq | gemini). The same set of
// clients backs both the keyword-naming and the judgement steps: each ai.Client implements every
// capability, so a mode map can be projected onto whichever interface a step needs. Pre-building all
// modes lets the dry-run endpoints switch backend per request for benchmarking without a restart.
func buildModeClients() map[string]ai.Client {
	return map[string]ai.Client{
		"local":  buildLocalClient(),
		"groq":   buildGroqClient(),
		"gemini": buildProvider("google", geminiModel),
	}
}

// buildKeyworders projects the mode clients onto the Keyworder interface, each wrapped with the
// verbose logger (KEYWORDS_VERBOSE_MODE), and returns them keyed by mode plus the configured default
// (KEYWORDS_MODE).
func buildKeyworders() (map[string]ai.Keyworder, string) {
	verbose := os.Getenv("KEYWORDS_VERBOSE_MODE") == "true"

	keyworders := make(map[string]ai.Keyworder)
	for mode, client := range buildModeClients() {
		keyworders[mode] = ai.NewVerboseKeyworder(client, "keywords/"+mode, verbose)
	}
	return keyworders, os.Getenv("KEYWORDS_MODE")
}

// buildJudgers projects the mode clients onto the Judger interface, each wrapped with the verbose
// logger (JUDGEMENT_VERBOSE_MODE), and returns them keyed by mode plus the configured default
// (JUDGEMENT_MODE).
func buildJudgers() (map[string]ai.Judger, string) {
	verbose := os.Getenv("JUDGEMENT_VERBOSE_MODE") == "true"

	judgers := make(map[string]ai.Judger)
	for mode, client := range buildModeClients() {
		judgers[mode] = ai.NewVerboseJudger(client, "judgement/"+mode, verbose)
	}
	return judgers, os.Getenv("JUDGEMENT_MODE")
}

func buildLocalClient() ai.Client {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	return openaicompat.NewClient(baseURL, localModel, "")
}

func buildGroqClient() ai.Client {
	apiKey := os.Getenv("GROQ_API_KEY")
	model := os.Getenv("GROQ_MODEL")
	if apiKey == "" || model == "" {
		log.Println("AI 'groq' mode disabled: set GROQ_API_KEY and GROQ_MODEL")
		return ai.NewDisabledClient()
	}
	baseURL := os.Getenv("GROQ_BASE_URL")
	if baseURL == "" {
		baseURL = defaultGroqBaseURL
	}
	return openaicompat.NewClient(baseURL, model, apiKey)
}

// parseConcurrency reads the discovery pipeline concurrency (how many sources / articles are
// processed in parallel). It falls back to sequential (1) when unset or invalid, since a
// misconfigured value must not crash the app. Dev keeps the default 1 (deterministic); staging and
// production raise it, bounded by the AI provider rate limit.
func parseConcurrency(s string) int {
	if s == "" {
		return defaultDiscoveryConcurrency
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		log.Printf("Invalid DISCOVERY_CONCURRENCY %q (want integer >= 1), using default %d", s, defaultDiscoveryConcurrency)
		return defaultDiscoveryConcurrency
	}
	return n
}

// parseMaxArticles reads the per-sweep cap on how many discovered items are handed to the pipeline.
// -1 (the default) means no cap — the production value. A positive value throttles AI usage while
// testing against a rate-limited provider. Invalid input falls back to -1 rather than crashing.
func parseMaxArticles(s string) int {
	if s == "" {
		return -1
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < -1 {
		log.Printf("Invalid DISCOVERY_MAX_ARTICLES %q (want integer >= -1), using default -1 (no cap)", s)
		return -1
	}
	return n
}

// parseSuggestionWindowDays reads how many days back the "popular" keyword-suggestion query looks.
// The default keeps "popular" meaning "popular recently"; -1 disables the window (all history).
// Invalid input falls back to the default rather than crashing.
func parseSuggestionWindowDays(s string) int {
	if s == "" {
		return defaultKeywordSuggestionsWindowDays
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < -1 {
		log.Printf("Invalid KEYWORD_SUGGESTIONS_WINDOW_DAYS %q (want integer >= -1), using default %d",
			s, defaultKeywordSuggestionsWindowDays)
		return defaultKeywordSuggestionsWindowDays
	}
	return n
}

// parseInactiveDays reads how many days without activity make a user "inactive" for the discovery
// filter. -1 disables the filter (every user is treated as active); a positive value is the window.
// 0 is rejected (it would freeze every feed immediately, which is never intended) and falls back to
// the default, as does any other invalid input.
func parseInactiveDays(s string) int {
	if s == "" {
		return defaultDaysUntilUserIsInactive
	}
	n, err := strconv.Atoi(s)
	if err != nil || n == 0 || n < -1 {
		log.Printf("Invalid DAYS_UNTIL_USER_IS_INACTIVE %q (want -1 to disable, or an integer >= 1), using default %d",
			s, defaultDaysUntilUserIsInactive)
		return defaultDaysUntilUserIsInactive
	}
	return n
}

// parseThreshold reads the judgement pass threshold (0-100). It falls back to a sane default when
// unset or invalid, since a misconfigured threshold must not crash the app.
func parseThreshold(s string) int {
	if s == "" {
		return defaultJudgementThreshold
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 100 {
		log.Printf("Invalid JUDGEMENT_THRESHOLD %q (want integer 0-100), using default %d", s, defaultJudgementThreshold)
		return defaultJudgementThreshold
	}
	return n
}

// parseAutoAssociateRatio reads the layer-2 auto-associate ratio (0-1): a candidate feed whose
// keywords are covered at least this fraction by the article is associated without an AI call. Falls
// back to the default when unset or out of range.
func parseAutoAssociateRatio(s string) float64 {
	if s == "" {
		return defaultJudgementAutoAssociateRatio
	}
	r, err := strconv.ParseFloat(s, 64)
	if err != nil || r < 0 || r > 1 {
		log.Printf("Invalid JUDGEMENT_AUTOASSOCIATE_RATIO %q (want float 0-1), using default %.2f", s, defaultJudgementAutoAssociateRatio)
		return defaultJudgementAutoAssociateRatio
	}
	return r
}

// parseMinMatches reads the minimum keyword overlap for a candidate to reach the AI (fewer than this
// is discarded without an AI call). Falls back to the default when unset or invalid.
func parseMinMatches(s string) int {
	if s == "" {
		return defaultJudgementMinMatches
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		log.Printf("Invalid JUDGEMENT_MIN_MATCHES %q (want integer >= 1), using default %d", s, defaultJudgementMinMatches)
		return defaultJudgementMinMatches
	}
	return n
}

// parseCSV splits a comma-separated env value into a trimmed, non-empty slice. Used for the OAuth
// redirect allowlist (OAUTH_ALLOWED_REDIRECT_URIS). An empty/blank value yields nil, which makes the
// login endpoint reject every redirect_uri until it is configured.
func parseCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// buildProvider wires an AI provider for a task from its configured provider/model. Supported
// providers come from conventions.md ("google" = Gemini LLM, "ollama" = local SLM via an
// OpenAI-compatible endpoint). When a provider is missing or misconfigured it returns a disabled
// client so the app still boots and serves non-AI features; AI-dependent paths then fail clearly
// instead of crashing at startup.
func buildProvider(provider, model string) ai.Client {
	switch provider {
	case "google":
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" || model == "" {
			log.Println("AI (google) disabled: set GOOGLE_API_KEY and the task model")
			return ai.NewDisabledClient()
		}
		client, err := gemini.NewClient(context.Background(), apiKey, model)
		if err != nil {
			log.Printf("Failed to initialize Gemini client, AI disabled: %v", err)
			return ai.NewDisabledClient()
		}
		return client
	case "ollama":
		if model == "" {
			log.Println("AI (ollama) disabled: set the task model")
			return ai.NewDisabledClient()
		}
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = defaultOllamaURL
		}
		return openaicompat.NewClient(baseURL, model, "")
	default:
		log.Printf("AI disabled: unknown provider %q (supported: google, ollama)", provider)
		return ai.NewDisabledClient()
	}
}

func runMigrations(database *sql.DB) error {
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set migration dialect: %w", err)
	}

	if err := goose.Up(database, "migrations"); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}

func parseMinutes(s string) (time.Duration, error) {
	minutes, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", s)
	}
	return time.Duration(minutes) * time.Minute, nil
}

func parseDays(s string) (time.Duration, error) {
	days, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", s)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
