-- +goose Up
-- +goose StatementBegin

-- Prevents importing the same Moxfield deck twice for the same user. Neither
-- the single-deck import nor the bulk import (internal/moxfieldimport)
-- checked for an existing row before inserting, so re-running either one
-- created a duplicate of a deck the user already had. Partial: moxfield_id
-- is NULL for manually-created decks, and multiple NULLs are never
-- duplicates anyway (same reasoning as decks_moxfield_id_idx, migration
-- 00003_indices.sql). Scoped per user_id, not moxfield_id alone: the same
-- public Moxfield deck can legitimately be imported by several different users.
CREATE UNIQUE INDEX decks_user_id_moxfield_id_unique_idx
  ON decks (user_id, moxfield_id) WHERE moxfield_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX decks_user_id_moxfield_id_unique_idx;

-- +goose StatementEnd
