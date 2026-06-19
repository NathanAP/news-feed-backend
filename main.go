package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

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

	app := fiber.New(fiber.Config{
		AppName: os.Getenv("PROJECT_NAME"),
	})

	apiVersion := os.Getenv("API_VERSION")
	api := app.Group("/" + apiVersion)

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": os.Getenv("PROJECT_VERSION"),
		})
	})

	log.Fatal(app.Listen(":3000"))
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
