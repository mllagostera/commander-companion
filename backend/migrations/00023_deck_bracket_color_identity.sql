-- +goose Up
-- +goose StatementBegin

-- A deck's Commander bracket (1-5, Wizards' power-level scale) and color
-- identity, so decks, statistics and game history can be filtered by them.
-- Both come from Moxfield on import/resync and can be set by hand on any deck;
-- a hand-set value is flagged as overridden and resyncs leave it alone.
--
-- color_identity holds the letters in WUBRG order: '{}' is colorless, NULL is
-- unknown (a deck imported before this migration that hasn't been resynced,
-- or a manual deck nobody filled in). Same for bracket.
ALTER TABLE decks
  ADD COLUMN bracket smallint CHECK (bracket BETWEEN 1 AND 5),
  ADD COLUMN color_identity text[] CHECK (color_identity <@ ARRAY['W', 'U', 'B', 'R', 'G']),
  ADD COLUMN bracket_overridden boolean NOT NULL DEFAULT false,
  ADD COLUMN color_identity_overridden boolean NOT NULL DEFAULT false;

-- What the deck was when it sat down at a game: a deck's bracket changes over
-- time, and statistics/history filter by what was actually played. Copied
-- from decks when the seat is added (games.AddGamePlayer).
ALTER TABLE game_players
  ADD COLUMN deck_bracket smallint CHECK (deck_bracket BETWEEN 1 AND 5),
  ADD COLUMN deck_color_identity text[] CHECK (deck_color_identity <@ ARRAY['W', 'U', 'B', 'R', 'G']);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE game_players
  DROP COLUMN deck_color_identity,
  DROP COLUMN deck_bracket;

ALTER TABLE decks
  DROP COLUMN color_identity_overridden,
  DROP COLUMN bracket_overridden,
  DROP COLUMN color_identity,
  DROP COLUMN bracket;

-- +goose StatementEnd
