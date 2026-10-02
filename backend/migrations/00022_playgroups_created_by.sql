-- +goose Up
-- +goose StatementBegin

-- Who created the group: the only user allowed to delete it (DELETE
-- /playgroups/{id}). Renaming and adding members stay open to every member.
-- ON DELETE SET NULL so removing a user never blocks on, or takes down, the
-- groups they created; a group with no creator simply can't be deleted.
ALTER TABLE playgroups ADD COLUMN created_by uuid REFERENCES users(id) ON DELETE SET NULL;

-- Existing groups never recorded their creator. CreatePlaygroup has always
-- added the creator as the first member right after inserting the group, so
-- the earliest member by joined_at is the creator (user_id breaks a tie only
-- to keep the backfill deterministic).
UPDATE playgroups p SET created_by = (
  SELECT pm.user_id FROM playgroup_members pm
  WHERE pm.playgroup_id = p.id
  ORDER BY pm.joined_at ASC NULLS LAST, pm.user_id
  LIMIT 1
);

-- FK index, same policy as 00014_deck_resync_rls_and_fk_indexes.sql.
CREATE INDEX playgroups_created_by_idx ON playgroups (created_by);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX playgroups_created_by_idx;
ALTER TABLE playgroups DROP COLUMN created_by;

-- +goose StatementEnd
