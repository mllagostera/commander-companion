-- +goose Up
-- +goose StatementBegin

-- Resincronización asíncrona de TODOS los decks ya importados de un usuario que
-- tengan moxfield_id (ver internal/deckresync) — distinto de moxfield_import_jobs,
-- que trae decks NUEVOS a partir de un username. Mismo shape (contadores
-- agregados, no un ítem por deck) que esa tabla, ver su comentario.
CREATE TABLE deck_resync_jobs (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid REFERENCES users(id),
  status          varchar NOT NULL DEFAULT 'pending',
  total_decks     int NOT NULL,
  updated_count   int NOT NULL DEFAULT 0,
  failed_count    int NOT NULL DEFAULT 0,
  error_message   varchar,
  created_at      timestamp DEFAULT (now()),
  updated_at      timestamp,
  finished_at     timestamp,
  CONSTRAINT deck_resync_jobs_status_chk
    CHECK (status IN ('pending', 'in_progress', 'completed', 'failed'))
);

-- Un solo job activo por usuario a la vez, mismo motivo que moxfield_import_jobs.
CREATE UNIQUE INDEX deck_resync_jobs_active_user_idx
  ON deck_resync_jobs (user_id) WHERE status IN ('pending', 'in_progress');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX deck_resync_jobs_active_user_idx;
DROP TABLE deck_resync_jobs;

-- +goose StatementEnd
