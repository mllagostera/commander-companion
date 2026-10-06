-- +goose Up
-- +goose StatementBegin

-- password_reset_tokens was created in 00024 without RLS, the same mistake
-- 00014 fixed for deck_resync_jobs. Supabase exposes the public schema through
-- PostgREST, and anon/authenticated hold full table grants by default, so with
-- the publishable key anyone could INSERT a token row of their own for any
-- user_id (or clear used_at on a spent one) and reset that user's password:
-- account takeover. Deny-all RLS (enabled, no policies) closes it; the backend
-- connects directly to Postgres as the table owner (DB_URL), which RLS does
-- not restrict.
ALTER TABLE password_reset_tokens ENABLE ROW LEVEL SECURITY;

-- Backfill: RLS on the tables created in 00001-00012 was only ever enabled by
-- hand in the Supabase dashboard, never in a migration, so any database built
-- from this repo started with them open. ENABLE is idempotent, so this is a
-- no-op where it was already on. check-architecture.sh now fails on any
-- CREATE TABLE without a matching ENABLE ROW LEVEL SECURITY.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_verification_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE decks ENABLE ROW LEVEL SECURITY;
ALTER TABLE playgroups ENABLE ROW LEVEL SECURITY;
ALTER TABLE playgroup_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE games ENABLE ROW LEVEL SECURITY;
ALTER TABLE game_players ENABLE ROW LEVEL SECURITY;
ALTER TABLE game_actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE commander_damage ENABLE ROW LEVEL SECURITY;
ALTER TABLE moxfield_import_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_statistics_summary ENABLE ROW LEVEL SECURITY;
ALTER TABLE deck_statistics_summary ENABLE ROW LEVEL SECURITY;
-- Created by goose itself, not by a migration, but exposed all the same.
ALTER TABLE goose_db_version ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Deliberately a no-op: disabling RLS here would reopen the hole above, and in
-- production it would also undo the dashboard-enabled RLS this migration only
-- made explicit.
SELECT 1;

-- +goose StatementEnd
