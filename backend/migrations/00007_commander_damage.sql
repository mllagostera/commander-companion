-- +goose Up
-- +goose StatementBegin

-- Daño de comandante acumulado por par atacante-defensor dentro de una partida
-- (regla real de Commander: 21+ de una misma fuente elimina, independiente del
-- life_total agregado). Ver internal/game-actions/service.go: applyCommanderDamage.
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
