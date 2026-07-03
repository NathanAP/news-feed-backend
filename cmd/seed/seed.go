package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
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

// openDB opens the application's SQLite database (same file as the API) and applies migrations, so
// the seed works even on a fresh database. A busy timeout reduces "database is locked" errors when
// the API happens to be running.
func openDB() (*sql.DB, error) {
	root, err := projectRoot()
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(root, "db", "news_feed.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	database, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to set migration dialect: %w", err)
	}
	if err := goose.Up(database, filepath.Join(root, "migrations")); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return database, nil
}
