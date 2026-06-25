package utils

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// SetupTestDB opens an in-memory SQLite database and runs all migrations.
// The database is closed automatically when the test ends.
func SetupTestDB(t testing.TB) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "failed to open test database")

	// An in-memory SQLite database lives per-connection. The database/sql pool may open
	// several connections, which would expose separate empty databases (a write on one
	// connection would be invisible to a read on another, breaking transactions). Pinning
	// the pool to a single connection keeps each test's in-memory DB consistent and isolated.
	db.SetMaxOpenConns(1)

	require.NoError(t, db.Ping(), "test database not reachable")

	migrationsPath := migrationsDir()

	goose.SetBaseFS(nil)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db, migrationsPath), "failed to apply migrations to test database")

	t.Cleanup(func() { db.Close() })

	return db
}

func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}
