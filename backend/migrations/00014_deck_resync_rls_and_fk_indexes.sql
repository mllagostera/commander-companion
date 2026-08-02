-- +goose Up
-- +goose StatementBegin

-- deck_resync_jobs quedó sin RLS al crearse (00013), a diferencia de toda otra
-- tabla del schema public. Como Supabase expone public vía PostgREST, eso deja
-- la tabla legible/escribible con la anon key. Se activa sin políticas, igual
-- que moxfield_import_jobs y el resto: deny-all para anon/authenticated, el
-- backend sigue accediendo por conexión directa a Postgres (DB_URL).
ALTER TABLE deck_resync_jobs ENABLE ROW LEVEL SECURITY;

-- Índices de cobertura para FKs que el advisor de performance de Supabase
-- marcó como unindexed_foreign_keys. Sin index de cobertura, un lookup o
-- borrado en cascada desde la tabla referenciada hace seq scan sobre estas.
CREATE INDEX commander_damage_defender_id_idx ON commander_damage (defender_id);
CREATE INDEX game_actions_actor_id_idx ON game_actions (actor_id);
CREATE INDEX game_actions_target_id_idx ON game_actions (target_id);
CREATE INDEX game_players_added_by_idx ON game_players (added_by);
CREATE INDEX game_players_deck_id_idx ON game_players (deck_id);
CREATE INDEX game_players_user_id_idx ON game_players (user_id);
CREATE INDEX games_current_turn_player_id_idx ON games (current_turn_player_id);
CREATE INDEX games_playgroup_id_idx ON games (playgroup_id);
CREATE INDEX playgroup_members_user_id_idx ON playgroup_members (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX playgroup_members_user_id_idx;
DROP INDEX games_playgroup_id_idx;
DROP INDEX games_current_turn_player_id_idx;
DROP INDEX game_players_user_id_idx;
DROP INDEX game_players_deck_id_idx;
DROP INDEX game_players_added_by_idx;
DROP INDEX game_actions_target_id_idx;
DROP INDEX game_actions_actor_id_idx;
DROP INDEX commander_damage_defender_id_idx;

ALTER TABLE deck_resync_jobs DISABLE ROW LEVEL SECURITY;

-- +goose StatementEnd
