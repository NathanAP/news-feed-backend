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

	database, err := sql.Open("sqlite", "./db/news_feed.db")
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

	treatmentProvider, treatmentModel := os.Getenv("TREATMENT_PROVIDER"), os.Getenv("TREATMENT_MODEL")
	var treater ai.Treater = buildProvider(treatmentProvider, treatmentModel)
	treater = ai.NewVerboseTreater(treater, fmt.Sprintf("%s/%s", treatmentProvider, treatmentModel), os.Getenv("TREATMENT_VERBOSE_MODE") == "true")

	keywordsProvider, keywordsModel := os.Getenv("KEYWORDS_PROVIDER"), os.Getenv("KEYWORDS_MODEL")
	var keyworder ai.Keyworder = buildProvider(keywordsProvider, keywordsModel)
	keyworder = ai.NewVerboseKeyworder(keyworder, fmt.Sprintf("%s/%s", keywordsProvider, keywordsModel), os.Getenv("KEYWORDS_VERBOSE_MODE") == "true")

	authMiddleware := middlewares.NewAuthMiddleware(jwtSecret, refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{
		AppName: os.Getenv("PROJECT_NAME"),
	})

	apiVersion := os.Getenv("API_VERSION")
	api := app.Group("/" + apiVersion)

	// Routes exempt from the maintenance guard. They must keep working while app_status is off:
	// /health for monitoring, and the toggle so the API can always be brought back online.
	api.Get("/health", healthendpoints.Check(systemCtrl, runTx))
	api.Put("/system/app-status", systemendpoints.UpdateAppStatus(systemCtrl, runTx))

	// Global maintenance guard: every route registered below returns 503 while app_status is
	// off. The two routes above are registered earlier and therefore stay reachable.
	api.Use(middlewares.NewAppStatusMiddleware(systemCtrl, runTx))

	auth := api.Group("/auth")
	auth.Get("/google", authendpoints.GoogleLogin(oauth2Config))
	auth.Get("/google/callback", authendpoints.GoogleCallback(authCtrl))
	auth.Post("/refresh", authendpoints.RefreshToken(authCtrl))
	auth.Post("/logout", append(authMiddleware, authendpoints.Logout(refreshTokenCtrl, runTx))...)
	auth.Delete("/invalidate", authendpoints.Invalidate(refreshTokenCtrl, runTx))
	auth.Delete("/invalidate-all", authendpoints.InvalidateAll(refreshTokenCtrl, runTx))

	users := api.Group("/users")
	users.Get("/me", append(authMiddleware, userendpoints.GetMe())...)
	users.Get("/me/preferences", append(authMiddleware, userendpoints.GetPreferences())...)
	users.Put("/me/preferences", append(authMiddleware, userendpoints.UpdatePreferences(prefCtrl, authCtrl, runTx, int(accessTokenExpiry.Seconds())))...)

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
	articles.Post("/treatment", append(authMiddleware, articleendpoints.TreatArticle(treater, keyworder))...)
	articles.Put("/:id/read", append(authMiddleware, articleendpoints.MarkAsRead(articleCtrl, afCtrl, runTx))...)
	articles.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, afCtrl, runTx))...)
	articles.Get("", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, runTx))...)
	articles.Put("/:id", append(authMiddleware, articleendpoints.UpdateArticle(articleCtrl, runTx))...)
	articles.Delete("/:id", append(authMiddleware, articleendpoints.DeleteArticle(articleCtrl, runTx))...)

	feeds := api.Group("/feeds")
	feeds.Post("/create", append(authMiddleware, feedendpoints.CreateFeed(feedCtrl, runTx))...)
	feeds.Get("/:id", append(authMiddleware, feedendpoints.GetFeed(feedCtrl, runTx))...)
	feeds.Get("", append(authMiddleware, feedendpoints.ListFeeds(feedCtrl, runTx))...)
	feeds.Put("/:id", append(authMiddleware, feedendpoints.UpdateFeed(feedCtrl, runTx))...)
	feeds.Delete("/:id", append(authMiddleware, feedendpoints.DeleteFeed(feedCtrl, runTx))...)

	if os.Getenv("RSS_FEED_CRON_ACTIVE") == "true" {
		cronVerbose := os.Getenv("RSS_FEED_CRON_VERBOSE_MODE") == "true"
		processor := discovery.NewTreatmentProcessor(runTx, articleCtrl, treater, keyworder, cronVerbose)
		runner := cron.NewDiscoveryRunner(runTx, sourceCtrl, systemCtrl, processor, discoveryHTTPClient, cronVerbose)
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
			baseURL = "http://localhost:11434"
		}
		return openaicompat.NewClient(baseURL, model)
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
