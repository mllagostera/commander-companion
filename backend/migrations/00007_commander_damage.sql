-- +goose Up
-- +goose StatementBegin

-- Commander damage accumulated per attacker-defender pair within a game
-- (real Commander rule: 21+ from the same source eliminates, independent of
-- aggregate life_total). See internal/game-actions/service.go: applyCommanderDamage.
CREATE TABLE commander_damage (
  game_id     uuid REFERENCES games(id),
  attacker_id uuid REFERENCES game_players(id),
  defender_id uuid REFERENCES game_players(id),
  amount      int NOT NULL DEFAULT 0,
  PRIMARY KEY (attacker_id, defender_id),
  CONSTRAINT commander_damage_distinct_players_chk CHECK (attacker_id <> defender_id),
  CONSTRAINT commander_damage_amount_chk CHECK (amount >= 0)
);

CREATE INDEX commander_damage_game_id_idx ON commander_damage (game_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX commander_damage_game_id_idx;
DROP TABLE commander_damage;

-- +goose StatementEnd
