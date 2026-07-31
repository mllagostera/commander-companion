package common

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxPoolConns    = 25
	minPoolConns    = 5
	maxConnLifetime = 30 * time.Minute
	maxConnIdleTime = 5 * time.Minute
	connectTimeout  = 5 * time.Second
)

// DB represents the connection to the database.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB initializes and returns a database connection using pgxpool.
func NewDB(databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing db config: %w", err)
	}

	// Production settings
	config.MaxConns = maxPoolConns
	config.MinConns = minPoolConns
	config.MaxConnLifetime = maxConnLifetime
	config.MaxConnIdleTime = maxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database is not reachable: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}
