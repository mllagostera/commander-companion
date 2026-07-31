// Package testutil provides shared helpers for integration tests that
// need a real PostgreSQL connection with the goose migrations schema
// already applied (same as the build-test-migrate job in backend-ci.yml).
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultTestDBURL points to a database separate from the development one
// (`commander`, see docker-compose.yml) so that running tests locally doesn't
// wipe out real data by accident. It matches the one the CI job uses.
//
//nolint:gosec // dev-only default credential, not a real secret (same pattern as cmd/api/main.go)
const defaultTestDBURL = "postgres://postgres:postgres@localhost:5432/commander_test?sslmode=disable"

const pingTimeout = 3 * time.Second

// DB opens a pool against the test database (env DB_URL, the same one
// backend-ci.yml sets) and skips the test if Postgres isn't available, so
// `go test ./...` doesn't fail for whoever doesn't have Docker/Postgres running
// locally. In CI, DB_URL is always set and Postgres is always available.
func DB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = defaultTestDBURL
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("skipping: no se pudo parsear DB_URL de test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping: base de datos de test no disponible en %s: %v", dbURL, err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// Truncate empties the given tables (with CASCADE, so it also sweeps ones
// with an FK pointing at them) before the test and when it finishes, so each test
// starts from a clean state without depending on execution order or on
// the previous process having finished cleanly.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()

	truncate := func() {
		ctx := context.Background()
		for _, tbl := range tables {
			if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" CASCADE"); err != nil {
				t.Fatalf("truncando %s: %v", tbl, err)
			}
		}
	}

	truncate()
	t.Cleanup(truncate)
}
