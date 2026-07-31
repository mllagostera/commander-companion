-- +goose Up
-- +goose StatementBegin

-- Bulk, asynchronous import of all of a Moxfield user's public decks
-- (see internal/moxfieldimport). Aggregate counters, not one item per deck: this is
-- enough for a progress bar; a moxfield_import_job_items table is left as a
-- future expansion if per-deck detail is ever needed.
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

-- Only one active job per user at a time, to prevent two concurrent
-- imports for the same user from hammering Moxfield in parallel.
CREATE UNIQUE INDEX moxfield_import_jobs_active_user_idx
  ON moxfield_import_jobs (user_id) WHERE status IN ('pending', 'in_progress');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX moxfield_import_jobs_active_user_idx;
DROP TABLE moxfield_import_jobs;

-- +goose StatementEnd
