-- +goose Up
-- +goose StatementBegin

-- Import masivo y asíncrono de todos los decks públicos de un usuario de Moxfield
-- (ver internal/moxfieldimport). Contadores agregados, no un ítem por deck: alcanza
-- para una barra de progreso; una tabla moxfield_import_job_items queda como
-- expansión futura si hace falta detalle por deck.
CREATE TABLE moxfield_import_jobs (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id           uuid REFERENCES users(id),
  moxfield_username varchar NOT NULL,
  status            varchar NOT NULL DEFAULT 'pending',
  total_decks       int,
  imported_count    int NOT NULL DEFAULT 0,
  failed_count      int NOT NULL DEFAULT 0,
  error_message     varchar,
  created_at        timestamp DEFAULT (now()),
  updated_at        timestamp,
  finished_at       timestamp,
  CONSTRAINT moxfield_import_jobs_status_chk
    CHECK (status IN ('pending', 'in_progress', 'completed', 'failed'))
);

-- Un solo job activo por usuario a la vez, para no permitir dos importaciones
-- concurrentes del mismo usuario machacando Moxfield en paralelo.
CREATE UNIQUE INDEX moxfield_import_jobs_active_user_idx
  ON moxfield_import_jobs (user_id) WHERE status IN ('pending', 'in_progress');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX moxfield_import_jobs_active_user_idx;
DROP TABLE moxfield_import_jobs;

-- +goose StatementEnd
