package main

import (
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
	"github.com/nathanap/news-feed-backend/services/controllers"
	articleendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/articles"
	authendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/auth"
	sourceendpoints "github.com/nathanap/news-feed-backend/services/endpoints/v1/sources"
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

	authMiddleware := middlewares.NewAuthMiddleware(jwtSecret, refreshTokenCtrl, runTx)

	app := fiber.New(fiber.Config{
		AppName: os.Getenv("PROJECT_NAME"),
	})

	apiVersion := os.Getenv("API_VERSION")
	api := app.Group("/" + apiVersion)

	api.Get("/health", func(c *fiber.Ctx) error {
		logger.RouteStart(c.Path())
		defer logger.RouteEnd(c.Path())
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": os.Getenv("PROJECT_VERSION"),
		})
	})

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

	sources := api.Group("/sources")
	sources.Post("/create", append(authMiddleware, sourceendpoints.CreateSource(sourceCtrl, runTx))...)
	sources.Get("/rss_discovery", append(authMiddleware, sourceendpoints.RSSDiscovery(&http.Client{}))...)
	sources.Get("/:id", append(authMiddleware, sourceendpoints.GetSource(sourceCtrl, runTx))...)
	sources.Get("", append(authMiddleware, sourceendpoints.ListSources(sourceCtrl, runTx))...)
	sources.Put("/:id", append(authMiddleware, sourceendpoints.UpdateSource(sourceCtrl, runTx))...)
	sources.Delete("/:id", append(authMiddleware, sourceendpoints.DeleteSource(sourceCtrl, runTx))...)

	articles := api.Group("/articles")
	articles.Post("/create", append(authMiddleware, articleendpoints.CreateArticle(articleCtrl, runTx))...)
	articles.Get("/:id", append(authMiddleware, articleendpoints.GetArticle(articleCtrl, runTx))...)
	articles.Get("", append(authMiddleware, articleendpoints.ListArticles(articleCtrl, runTx))...)
	articles.Put("/:id", append(authMiddleware, articleendpoints.UpdateArticle(articleCtrl, runTx))...)
	articles.Delete("/:id", append(authMiddleware, articleendpoints.DeleteArticle(articleCtrl, runTx))...)

	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "3000"
	}

	log.Fatal(app.Listen(":" + apiPort))
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
