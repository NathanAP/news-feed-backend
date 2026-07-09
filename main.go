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
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	_ "modernc.org/sqlite"

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
	"github.com/nathanap/news-feed-backend/services/sanitize"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	logger.Setup()

	if err := os.MkdirAll("db", os.ModePerm); err != nil {
		log.Fatalf("Failed to create db directory: %v", err)
	}

	// SQLite connection pragmas, carried on the DSN so every pooled connection inherits them:
	//   busy_timeout(5000): wait up to 5s for a lock instead of failing immediately with SQLITE_BUSY.
	//     SQLite allows a single writer at a time, and the discovery pipeline (DISCOVERY_CONCURRENCY)
	//     plus the cron running alongside API requests can contend for it — without this, the losing
	//     writer errors out and the article is dropped.
	//   journal_mode(WAL): readers no longer block the writer, so API reads proceed during a
	//     discovery sweep. Both are SQLite-specific DSN params — a future Postgres swap drops them.
	database, err := sql.Open("sqlite", "./db/news_feed.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
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
	afCtrl := controllers.NewArticleFeedController()
	feedCtrl := controllers.NewFeedController()
	systemCtrl := controllers.NewSystemController()

	// TREATMENT_AI_ACTIVE=false skips only the LLM review step of treatment: the original RSS content
	// flows forward via a passthrough treater. Sanitize still wraps it, so the HTML whitelist is
	// enforced either way. The dev-only dry-run endpoint reuses this same `treater`, so it honors the
	// flag automatically.
	treatmentProvider, treatmentModel := os.Getenv("TREATMENT_PROVIDER"), os.Getenv("TREATMENT_MODEL")
	var treater ai.Treater
	treatmentLabel := fmt.Sprintf("%s/%s", treatmentProvider, treatmentModel)
	if os.Getenv("TREATMENT_AI_ACTIVE") == "true" {
		treater = buildProvider(treatmentProvider, treatmentModel)
	} else {
		treater = ai.NewPassthroughTreater()
		treatmentLabel = "passthrough (TREATMENT_AI_ACTIVE=false)"
	}
	treater = sanitize.NewTreater(treater) // enforce the basic-HTML whitelist on the treated output
	treater = ai.NewVerboseTreater(treater, treatmentLabel, os.Getenv("TREATMENT_VERBOSE_MODE") == "true")

	keyworders, keywordsDefaultMode := buildKeyworders()
	defaultKeyworder := keyworders[keywordsDefaultMode]
	if defaultKeyworder == nil {
		log.Printf("KEYWORDS_MODE %q not recognized (use local, groq or gemini); keywords disabled by default", keywordsDefaultMode)
		defaultKeyworder = ai.NewDisabledClient()
	}

	judgers, judgementDefaultMode := buildJudgers()
	judgementThreshold := parseThreshold(os.Getenv("JUDGEMENT_THRESHOLD"))
	defaultJudger := judgers[judgementDefaultMode]
	if defaultJudger == nil {
		log.Printf("JUDGEMENT_MODE %q not recognized (use local, groq or gemini); judgement disabled by default", judgementDefaultMode)
		defaultJudger = ai.NewDisabledClient()
	}
	evaluator := judgement.NewEvaluator(defaultJudger, judgementThreshold)

	// Language detection (lingua-go) is used by the treatment step; not AI, so no provider/mode.
	detector := langdetect.New()

	// Translation is LLM-only (quality-sensitive user-facing text): a single provider via TRANSLATION_*.
	translationProvider, translationModel := os.Getenv("TRANSLATION_PROVIDER"), os.Getenv("TRANSLATION_MODEL")
	var translator ai.Translator = buildProvider(translationProvider, translationModel)
	translator = ai.NewVerboseTranslator(translator, fmt.Sprintf("%s/%s", translationProvider, translationModel), os.Getenv("TRANSLATION_VERBOSE_MODE") == "true")

	authMiddleware := middlewares.NewAuthMiddleware(jwtSecret, refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{
		AppName: os.Getenv("PROJECT_NAME"),
	})

	// CORS is mounted first, before every route and guard, so it also covers the preflight OPTIONS
	// and the maintenance/health responses. The web client lives on a different origin, so without
	// this the browser blocks all cross-origin calls.
	app.Use(middlewares.NewCORSMiddleware())

	apiVersion := os.Getenv("API_VERSION")
	api := app.Group("/" + apiVersion)

	// Routes exempt from the maintenance guard. They must keep working while app_status is off:
	// /health for monitoring, and the toggle so the API can always be brought back online.
	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))
	api.Put("/system/app-status", systemendpoints.UpdateAppStatus(systemCtrl, runTx))

	// Global maintenance guard: every route registered below returns 503 while app_status is
	// off. The two routes above are registered earlier and therefore stay reachable.
	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx))

	oauthRedirectAllowlist := parseCSV(os.Getenv("OAUTH_ALLOWED_REDIRECT_URIS"))

	auth := api.Group("/auth")
	auth.Get("/google", authendpoints.GoogleLogin(oauth2Config, jwtSecret, oauthRedirectAllowlist))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl, jwtSecret))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl, runTx))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(refreshTokenCtrl, runTx))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(refreshTokenCtrl, runTx))

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

	sources := api.Group("/sources")
	sources.Post("/create", append(authMiddleware, sourceendpoints.CreateSource(sourceCtrl, runTx))...)
	sources.Get("/rss-discovery", append(authMiddleware, sourceendpoints.RSSDiscovery(&http.Client{}))...)
	sources.Get("/:id/article-discovery", append(authMiddleware, sourceendpoints.SourceArticleDiscovery(sourceCtrl, articleCtrl, runTx, discoveryHTTPClient))...)
	sources.Get("/:id", append(authMiddleware, sourceendpoints.GetSource(sourceCtrl, runTx))...)
	sources.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)
	sources.Put("/:id", append(authMiddleware, sourceendpoints.UpdateSource(sourceCtrl, runTx))...)
	sources.Delete("/:id", append(authMiddleware, sourceendpoints.DeleteSource(sourceCtrl, runTx))...)

	articles := api.Group("/articles")
	articles.Post("/create", append(authMiddleware, articleendpoints.CreateArticle(articleCtrl, runTx))...)
	articles.Get("/:id/translate/:language", append(authMiddleware, articleendpoints.TranslateArticle(articleCtrl, translator, runTx))...)
	articles.Put("/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(articleCtrl, afCtrl, runTx))...)
	articles.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, afCtrl, runTx))...)
	articles.Get("", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, runTx))...)
	articles.Put("/:id", append(authMiddleware, articleendpoints.UpdateArticle(articleCtrl, runTx))...)
	articles.Delete("/:id", append(authMiddleware, articleendpoints.DeleteArticle(articleCtrl, runTx))...)

	// Development-only dry-run tools for the AI pipeline (treatment, judgement). They call the AI
	// for real (consume quota) and expose internal pipeline behavior, so they must never be reachable
	// by clients in staging/production. Registered conditionally, like /users/dev-login, so the routes
	// literally do not exist outside development (defense in depth beyond any runtime check).
	if os.Getenv("ENVIRONMENT") == "development" {
		articles.Post("/treatment", append(authMiddleware, articleendpoints.TreatArticle(treater, keyworders, keywordsDefaultMode, detector))...)
		articles.Post("/judgement", append(authMiddleware, articleendpoints.JudgeArticle(feedCtrl, judgers, judgementDefaultMode, judgementThreshold, runTx))...)
		log.Println("Development mode: POST /v1/articles/treatment and /v1/articles/judgement enabled")
	}

	feeds := api.Group("/feeds")
	feeds.Post("/create", append(authMiddleware, feedendpoints.CreateFeed(feedCtrl, runTx))...)
	// Static route registered before "/:id" so "check-for-new-articles" is never swallowed as an id
	// (Fiber prioritizes static over param, but keeping the order explicit matches sources/rss-discovery).
	feeds.Get("/check-for-new-articles", append(authMiddleware, feedendpoints.CheckForNewArticles(afCtrl, runTx))...)
	feeds.Get("/:id/articles", append(authMiddleware, feedendpoints.FeedArticles(feedCtrl, afCtrl, runTx))...)
	feeds.Get("/:id", append(authMiddleware, feedendpoints.GetFeed(feedCtrl, runTx))...)
	feeds.Get("", append(authMiddleware, feedendpoints.ListFeeds(feedCtrl, runTx))...)
	feeds.Put("/:id", append(authMiddleware, feedendpoints.UpdateFeed(feedCtrl, runTx))...)
	feeds.Delete("/:id", append(authMiddleware, feedendpoints.DeleteFeed(feedCtrl, runTx))...)

	if os.Getenv("RSS_FEED_CRON_ACTIVE") == "true" {
		cronVerbose := os.Getenv("RSS_FEED_CRON_VERBOSE_MODE") == "true"
		discoveryConcurrency := parseConcurrency(os.Getenv("DISCOVERY_CONCURRENCY"))
		processor := discovery.NewTreatmentProcessor(runTx, articleCtrl, feedCtrl, afCtrl, detector, treater, defaultKeyworder, evaluator, discoveryConcurrency, cronVerbose)
		runner := cron.NewDiscoveryRunner(runTx, sourceCtrl, systemCtrl, processor, discoveryHTTPClient, discoveryConcurrency, cronVerbose)
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

// SLM/LLM models are fixed to the ones declared in the stack (CLAUDE.md); the Groq model is
// configurable because the stack does not pin a specific one.
const (
	localModel                  = "qwen3:4b"
	geminiModel                 = "gemini-2.5-flash"
	defaultGroqBaseURL          = "https://api.groq.com/openai/v1"
	defaultOllamaURL            = "http://localhost:11434"
	defaultJudgementThreshold   = 70
	defaultDiscoveryConcurrency = 1
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

	if err := goose.SetDialect("sqlite3"); err != nil {
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
