package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("narayana_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		fmt.Printf("Failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Printf("Failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	// Create connection pool
	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Printf("Failed to create pool: %v\n", err)
		os.Exit(1)
	}

	// Run migrations using raw SQL (avoids golang-migrate file:// URL issues on Windows)
	if err := runMigrations(ctx); err != nil {
		fmt.Printf("Failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	testPool.Close()
	pgContainer.Terminate(ctx)

	os.Exit(code)
}

// runMigrations reads and executes all *.up.sql files in order.
func runMigrations(ctx context.Context) error {
	migrationsDir := filepath.Join("..", "..", "db", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	// Collect and sort up migration files
	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return fmt.Errorf("reading %s: %w", f, err)
		}
		if _, err := testPool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("executing %s: %w", f, err)
		}
		fmt.Printf("  Migration applied: %s\n", f)
	}

	return nil
}

// truncateExecutions clears the executions table between tests.
func truncateExecutions(ctx context.Context) error {
	_, err := testPool.Exec(ctx, "TRUNCATE TABLE executions")
	return err
}

// truncateAll clears all tables between tests.
func truncateAll(ctx context.Context) error {
	_, err := testPool.Exec(ctx, "TRUNCATE TABLE execution_transitions, executions")
	return err
}
