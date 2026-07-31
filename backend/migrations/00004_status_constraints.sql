-- +goose Up
-- +goose StatementBegin

-- games.status: state machine enforced server-side in internal/games/service.go
-- (statusPending/statusActive/statusFinished). CHECK instead of a Postgres enum so as not
-- to break the Go type sqlc generates for the column (it stays string/varchar).
ALTER TABLE games
  ADD CONSTRAINT games_status_chk
  CHECK (status IN ('pending', 'active', 'finished'));

-- game_actions.action_type: fixed vocabulary validated in internal/game-actions/service.go
-- (isValidActionType). Same approach: CHECK on the existing varchar, no enum.
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
