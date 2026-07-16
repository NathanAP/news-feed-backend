package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

// devEnvironment is the only ENVIRONMENT in which the seed commands may run.
const devEnvironment = "development"

// report accumulates a per-command summary of what was created vs. skipped, printed at the end so
// the operator sees exactly what happened (idempotent commands skip existing records and say so).
type report struct {
	created []string
	skipped []string
}

func (r *report) add(msg string)  { r.created = append(r.created, msg) }
func (r *report) skip(msg string) { r.skipped = append(r.skipped, msg) }

func (r *report) print(command string) {
	fmt.Printf("\n=== seed: %s ===\n", command)
	fmt.Printf("created (%d):\n", len(r.created))
	for _, m := range r.created {
		fmt.Printf("  + %s\n", m)
	}
	fmt.Printf("skipped (%d):\n", len(r.skipped))
	for _, m := range r.skipped {
		fmt.Printf("  ~ %s\n", m)
	}
	fmt.Println()
}

// requireDevEnvironment loads the .env and aborts unless ENVIRONMENT=development. The seed writes
// fake data and must never touch staging/production.
func requireDevEnvironment() error {
	// Best-effort: env vars may already be set without a .env file.
	_ = godotenv.Load(filepath.Join(mustProjectRoot(), ".env"))

	if env := os.Getenv("ENVIRONMENT"); env != devEnvironment {
		return fmt.Errorf("seed commands only run when ENVIRONMENT=%s (got %q)", devEnvironment, env)
	}
	return nil
}

// projectRoot walks up from the working directory until it finds go.mod, so the seed resolves the
// DB file and migrations regardless of which directory `go run`/`task` invoked it from.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found walking up from the working directory")
		}
		dir = parent
	}
}

func mustProjectRoot() string {
	root, err := projectRoot()
	if err != nil {
		return "."
	}
	return root
}

// openDB connects to the same Postgres database the API uses (DATABASE_URL) and applies migrations,
// so the seed works even against a database that has never been migrated. Unlike the API, the seed
// reads the migrations from disk rather than the embedded FS, since it runs from the repository.
func openDB() (*sql.DB, error) {
	root, err := projectRoot()
	if err != nil {
		return nil, err
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be set")
	}

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to set migration dialect: %w", err)
	}
	if err := goose.Up(database, filepath.Join(root, "migrations")); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return database, nil
}
