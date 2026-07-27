-- +goose Up
-- +goose StatementBegin

-- games.status: máquina de estados aplicada server-side en internal/games/service.go
-- (statusPending/statusActive/statusFinished). CHECK en vez de enum de Postgres para no
-- romper el tipo Go generado por sqlc para la columna (seguiría siendo string/varchar).
ALTER TABLE games
  ADD CONSTRAINT games_status_chk
  CHECK (status IN ('pending', 'active', 'finished'));

-- game_actions.action_type: vocabulario fijo validado en internal/game-actions/service.go
-- (isValidActionType). Mismo criterio: CHECK sobre el varchar existente, no enum.
ALTER TABLE game_actions
  ADD CONSTRAINT game_actions_action_type_chk
  CHECK (action_type IN (
    'LifeChange',
    'CombatDamage',
    'CommanderDamage',
    'PoisonCounter',
    'TurnStart',
    'TurnEnd',
    'Elimination'
  ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE game_actions
  DROP CONSTRAINT game_actions_action_type_chk;

ALTER TABLE games
  DROP CONSTRAINT games_status_chk;

-- +goose StatementEnd
