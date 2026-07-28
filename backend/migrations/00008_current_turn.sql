-- +goose Up
-- +goose StatementBegin

-- Quién tiene el turno ahora mismo en la partida. Nullable: sin turno asignado
-- hasta el primer TurnStart. No modela "orden de turno" (quién sigue) -- solo
-- responde "de quién es el turno ahora", que es lo único que hoy usan
-- TurnStart/TurnEnd (internal/game-actions/service.go).
ALTER TABLE games ADD COLUMN current_turn_player_id uuid REFERENCES game_players(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE games DROP COLUMN current_turn_player_id;

-- +goose StatementEnd
