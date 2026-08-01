package common

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// RunMigrations applies pending goose migrations in dir against
// databaseURL. It runs embedded in the binary itself (instead of a
// separate "release command" step) because some deployment environments
// (e.g. Render's free tier) don't offer that hook — this way the same
// binary that serves the API brings the schema up to date before
// starting, on any platform.
//
// It uses a database/sql connection (via pgx's stdlib driver) separate from
// the app's pgxpool: goose doesn't speak pgxpool's native protocol, and this
// connection is closed as soon as it's done, before opening the real pool.
//
// A Postgres session-level advisory lock (goose's SessionLocker) serializes
// this call across replicas: if the service ever runs with more than one
// instance, whichever loses the race blocks until the winner finishes and
// releases the lock, instead of both running `goose up` concurrently.
func RunMigrations(databaseURL, dir string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("abriendo conexión para migraciones: %w", err)
	}
	defer func() { _ = db.Close() }()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("creando el locker de migraciones: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(dir), goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("creando el provider de migraciones: %w", err)
	}

	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("aplicando migraciones: %w", err)
	}

	return nil
}
