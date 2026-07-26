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

// DB representa la conexión a la base de datos.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB inicializa y devuelve una conexión a la base de datos usando pgxpool.
func NewDB(databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing db config: %w", err)
	}

	// Configuraciones para producción
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

// Close cierra el pool de conexiones.
func (db *DB) Close() {
	db.Pool.Close()
}
