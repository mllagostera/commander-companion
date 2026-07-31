package common

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
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
func RunMigrations(databaseURL, dir string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("abriendo conexión para migraciones: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configurando dialecto de goose: %w", err)
	}

	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("aplicando migraciones: %w", err)
	}

	return nil
}
