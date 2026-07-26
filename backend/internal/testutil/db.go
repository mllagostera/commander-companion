// Package testutil provee helpers compartidos para tests de integración que
// necesitan una conexión real a PostgreSQL con el schema de las migraciones
// de goose ya aplicado (igual que el job build-test-migrate de backend-ci.yml).
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultTestDBURL apunta a una base separada de la de desarrollo
// (`commander`, ver docker-compose.yml) para que correr los tests en local no
// arrase datos reales por accidente. Coincide con la que usa el job de CI.
//
//nolint:gosec // dev-only default credential, not a real secret (same pattern as cmd/api/main.go)
const defaultTestDBURL = "postgres://postgres:postgres@localhost:5432/commander_test?sslmode=disable"

const pingTimeout = 3 * time.Second

// DB abre un pool contra la base de test (env DB_URL, la misma que setea
// backend-ci.yml) y hace skip del test si Postgres no está disponible, para
// que `go test ./...` no falle para quien no tenga Docker/Postgres corriendo
// en local. En CI, DB_URL siempre está seteada y Postgres siempre disponible.
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

// Truncate vacía las tablas indicadas (con CASCADE, así arrastra las que
// tengan FK hacia ellas) antes del test y al terminar, para que cada test
// arranque de un estado limpio sin depender del orden de ejecución ni de que
// el proceso anterior haya terminado prolijo.
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
