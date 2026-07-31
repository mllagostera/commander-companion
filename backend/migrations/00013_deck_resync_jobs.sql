-- +goose Up
-- +goose StatementBegin

-- Asynchronous re-sync of ALL of a user's already-imported decks that
-- have a moxfield_id (see internal/deckresync) — different from moxfield_import_jobs,
-- which brings in NEW decks from a username. Same shape (aggregate
-- counters, not one item per deck) as that table, see its comment.
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

-- Only one active job per user at a time, same reason as moxfield_import_jobs.
CREATE UNIQUE INDEX deck_resync_jobs_active_user_idx
  ON deck_resync_jobs (user_id) WHERE status IN ('pending', 'in_progress');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX deck_resync_jobs_active_user_idx;
DROP TABLE deck_resync_jobs;

-- +goose StatementEnd
