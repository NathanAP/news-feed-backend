package utils

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Postgres has no in-memory mode, so the isolation rule ("each test gets its own database, and
// everything it creates is thrown away") is met differently than it was under SQLite: one throwaway
// Postgres container backs the whole run, migrations are applied once into a template database, and
// every test clones that template into a database of its own. Cloning is a file copy inside the
// server, so a test pays microseconds instead of a full goose migration.
//
// The container is named and reused (testcontainers' Reuse) rather than one per package: `go test`
// runs each package as its own process and runs packages in parallel, so without reuse the 14
// packages that touch the database would each boot their own Postgres — 14 at once on the dev
// machine. With reuse they share exactly one. Testcontainers' reaper still tears it down when the
// run ends, so nothing survives the suite; `task test-db-clean` only matters if a run is killed
// hard enough that the reaper does not get to it.
const (
	testContainerName = "news-feed-test-postgres"
	testImage         = "postgres:18-alpine"
	adminUser         = "postgres"
	adminPassword     = "postgres"
	adminDB           = "postgres"
	templateDB        = "news_feed_template"
	// Guards template creation across processes: the packages run in parallel and would otherwise
	// race to create and migrate the template. Any constant works; it only has to be shared.
	templateLockID = 8675309
	// How many times to retry joining the shared container. See runContainerWithRetry.
	containerStartAttempts = 5
)

var (
	setupOnce sync.Once
	adminDSN  string
	setupErr  error
)

// SetupTestDB gives the test its own Postgres database, cloned from a template that already has the
// migrations applied. The database is dropped when the test ends, so nothing a test writes can be
// seen by another one. Requires Docker to be running.
func SetupTestDB(t testing.TB) *sql.DB {
	t.Helper()

	setupOnce.Do(func() { adminDSN, setupErr = startContainerAndTemplate() })
	require.NoError(t, setupErr, "failed to prepare the test Postgres container")

	// A fresh name per test. UUIDs carry dashes, which would need quoting in an identifier, so they
	// are stripped rather than quoted.
	dbName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	admin, err := sql.Open("pgx", adminDSN)
	require.NoError(t, err, "failed to connect to the maintenance database")
	defer admin.Close()

	// CREATE DATABASE cannot run inside a transaction, hence the bare Exec. The template is never
	// connected to (only cloned from), so concurrent clones do not block each other.
	_, err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", dbName, templateDB))
	require.NoError(t, err, "failed to clone the template database")

	database, err := sql.Open("pgx", dsnFor(dbName))
	require.NoError(t, err, "failed to open test database")
	require.NoError(t, database.Ping(), "test database not reachable")

	t.Cleanup(func() {
		database.Close()

		cleanupAdmin, err := sql.Open("pgx", adminDSN)
		if err != nil {
			return
		}
		defer cleanupAdmin.Close()
		// WITH (FORCE) terminates any connection the test left behind, so a leaked handle cannot
		// keep the database alive and leak it into the next run.
		_, _ = cleanupAdmin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
	})

	return database
}

// startContainerAndTemplate boots (or reuses) the container and makes sure the migrated template
// database exists. It runs once per test process; the advisory lock makes it safe across processes.
func startContainerAndTemplate() (string, error) {
	ctx := context.Background()

	pinDockerHost()

	container, err := runContainerWithRetry(ctx)
	if err != nil {
		return "", err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read the container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return "", fmt.Errorf("failed to read the container port: %w", err)
	}

	admin := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		adminUser, adminPassword, host, port.Port(), adminDB)

	if err := ensureTemplate(admin); err != nil {
		return "", err
	}
	return admin, nil
}

// pinDockerHost makes sure DOCKER_HOST is set before testcontainers looks for the daemon.
//
// Without it, testcontainers walks a chain of fallbacks to find the endpoint — its properties file,
// the docker context, the default socket — and the last link is rootless detection, which is a hard
// error on Windows ("rootless Docker is not supported on Windows"). `go test` runs the packages in
// parallel, so a dozen processes read the docker context at the same instant; when that read loses
// the race the chain runs to the end and the whole package fails at setup. It is intermittent by
// nature: the suite passed several times before this surfaced.
//
// Asking Docker itself for the endpoint keeps this cross-platform (npipe on Windows, a unix socket
// elsewhere) instead of hardcoding a path per OS. If the lookup fails we leave DOCKER_HOST alone and
// let testcontainers try its chain — no worse than before.
func pinDockerHost() {
	if os.Getenv("DOCKER_HOST") != "" {
		return
	}
	out, err := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return
	}
	if host := strings.TrimSpace(string(out)); host != "" {
		os.Setenv("DOCKER_HOST", host)
	}
}

// runContainerWithRetry starts (or joins) the shared container, retrying a few times. The retry
// covers the other side of the same parallel burst: two processes can race to create the container
// and one loses with a name conflict. When Docker is genuinely down every attempt fails and the last
// error surfaces.
func runContainerWithRetry(ctx context.Context) (*postgres.PostgresContainer, error) {
	var lastErr error
	for attempt := 0; attempt < containerStartAttempts; attempt++ {
		container, err := postgres.Run(ctx, testImage,
			postgres.WithDatabase(adminDB),
			postgres.WithUsername(adminUser),
			postgres.WithPassword(adminPassword),
			testcontainers.WithReuseByName(testContainerName),
			testcontainers.WithWaitStrategy(
				wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
			),
		)
		if err == nil {
			return container, nil
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	return nil, fmt.Errorf("failed to start the postgres container after %d attempts (is Docker running?): %w",
		containerStartAttempts, lastErr)
}

// ensureTemplate creates and migrates the template database exactly once, even when several test
// packages reach this code at the same time: the advisory lock serializes them, and whoever loses
// the race finds the template already there and returns.
func ensureTemplate(adminDSN string) error {
	db, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to the maintenance database: %w", err)
	}
	defer db.Close()

	// Retry the first connection: with Reuse, a process may arrive while the container is still
	// accepting no connections.
	var pingErr error
	for i := 0; i < 30; i++ {
		if pingErr = db.Ping(); pingErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if pingErr != nil {
		return fmt.Errorf("test postgres not reachable: %w", pingErr)
	}

	// Session-level lock held until this connection is released. Taken on a dedicated connection so
	// the pool cannot hand the unlock to a different backend.
	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire a connection for the template lock: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock($1)", templateLockID); err != nil {
		return fmt.Errorf("failed to take the template lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", templateLockID)
	}()

	var exists bool
	if err := conn.QueryRowContext(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", templateDB).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check for the template database: %w", err)
	}
	if exists {
		return nil
	}

	if _, err := conn.ExecContext(context.Background(), "CREATE DATABASE "+templateDB); err != nil {
		return fmt.Errorf("failed to create the template database: %w", err)
	}

	templateConn, err := sql.Open("pgx", dsnForAdmin(adminDSN, templateDB))
	if err != nil {
		return fmt.Errorf("failed to connect to the template database: %w", err)
	}
	defer templateConn.Close()

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set the migration dialect: %w", err)
	}
	if err := goose.Up(templateConn, migrationsDir()); err != nil {
		return fmt.Errorf("failed to migrate the template database: %w", err)
	}
	return nil
}

// dsnFor builds the DSN of a database on the test container, reusing the host/port the container
// was published on.
func dsnFor(dbName string) string {
	return dsnForAdmin(adminDSN, dbName)
}

// dsnForAdmin swaps the database name of an existing DSN, so callers never rebuild host/port by hand.
func dsnForAdmin(dsn, dbName string) string {
	base, _, _ := strings.Cut(dsn, "?")
	base = base[:strings.LastIndex(base, "/")]
	return fmt.Sprintf("%s/%s?sslmode=disable", base, dbName)
}

func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}
