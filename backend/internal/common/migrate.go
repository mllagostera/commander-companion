package common

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// RunMigrations aplica las migraciones de goose pendientes en dir contra
// databaseURL. Se corre embebida en el propio binario (en vez de un paso
// separado tipo "release command") porque algunos entornos de despliegue
// (ej. el free tier de Render) no ofrecen ese hook — así el mismo binario
// que sirve la API deja el schema al día antes de arrancar, en cualquier
// plataforma.
//
// Usa una conexión database/sql (vía el driver stdlib de pgx) separada del
// pgxpool de la app: goose no habla el protocolo nativo de pgxpool, y esta
// conexión se cierra apenas termina, antes de abrir el pool real.
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
