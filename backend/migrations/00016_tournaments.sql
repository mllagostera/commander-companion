-- +goose Up
-- +goose StatementBegin

-- Swiss-format Commander tournaments, standalone (not tied to a playgroup): an
-- organizer creates one, participants (app users with one of their own decks, or
-- guests with no account) register via a join code, the organizer starts it once
-- the roster is ready, and each round's pods (3-4 seats) are paired by
-- internal/tournaments/pairing.go. Deliberately NOT wired into games/game_players
-- (the live life-tracker engine): a table's result is recorded manually by the
-- organizer, see internal/tournaments's package doc and ADR-0016.
CREATE TABLE tournaments (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organizer_id   uuid NOT NULL REFERENCES users(id),
  name           varchar NOT NULL,
  format         varchar NOT NULL DEFAULT 'commander',
  target_players int,
  status         varchar NOT NULL DEFAULT 'registration',
  round_count    int,
  current_round  int NOT NULL DEFAULT 0,
  join_code      varchar NOT NULL,
  created_at     timestamp DEFAULT (now()),
  started_at     timestamp,
  finished_at    timestamp,
  CONSTRAINT tournaments_format_chk CHECK (format = 'commander'),
  CONSTRAINT tournaments_status_chk CHECK (status IN ('registration', 'in_progress', 'finished'))
);

CREATE UNIQUE INDEX tournaments_join_code_idx ON tournaments (join_code);
CREATE INDEX tournaments_organizer_id_idx ON tournaments (organizer_id);

-- A participant is either an app user (user_id set, brings one of their own decks
-- via deck_id) or a guest with no account (guest_name set, commander_name free
-- text — decks belong to users, a guest has no decks row to point at).
CREATE TABLE tournament_participants (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tournament_id   uuid NOT NULL REFERENCES tournaments(id),
  user_id         uuid REFERENCES users(id),
  guest_name      varchar,
  deck_id         uuid REFERENCES decks(id),
  commander_name  varchar,
  points          int NOT NULL DEFAULT 0,
  created_at      timestamp DEFAULT (now()),
  CONSTRAINT tournament_participants_identity_chk CHECK (
    (user_id IS NOT NULL AND guest_name IS NULL) OR
    (user_id IS NULL AND guest_name IS NOT NULL)
  ),
  CONSTRAINT tournament_participants_commander_chk CHECK (
    deck_id IS NOT NULL OR commander_name IS NOT NULL
  )
);

-- An app user can only be registered once per tournament; guests have no such
-- constraint (user_id NULL rows are simply excluded by the partial index).
CREATE UNIQUE INDEX tournament_participants_unique_user_idx
  ON tournament_participants (tournament_id, user_id) WHERE user_id IS NOT NULL;
CREATE INDEX tournament_participants_tournament_id_idx ON tournament_participants (tournament_id);
CREATE INDEX tournament_participants_deck_id_idx ON tournament_participants (deck_id);

CREATE TABLE tournament_rounds (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tournament_id uuid NOT NULL REFERENCES tournaments(id),
  round_number  int NOT NULL,
  status        varchar NOT NULL DEFAULT 'pending',
  created_at    timestamp DEFAULT (now()),
  finished_at   timestamp,
  CONSTRAINT tournament_rounds_status_chk CHECK (status IN ('pending', 'finished')),
  CONSTRAINT tournament_rounds_unique_number UNIQUE (tournament_id, round_number)
);

CREATE INDEX tournament_rounds_tournament_id_idx ON tournament_rounds (tournament_id);

CREATE TABLE tournament_tables (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tournament_round_id  uuid NOT NULL REFERENCES tournament_rounds(id),
  table_number         int NOT NULL,
  CONSTRAINT tournament_tables_unique_number UNIQUE (tournament_round_id, table_number)
);

CREATE INDEX tournament_tables_round_id_idx ON tournament_tables (tournament_round_id);

CREATE TABLE tournament_table_seats (
  id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tournament_table_id         uuid NOT NULL REFERENCES tournament_tables(id),
  tournament_participant_id   uuid NOT NULL REFERENCES tournament_participants(id),
  finish_position             int,
  points_awarded              int NOT NULL DEFAULT 0,
  CONSTRAINT tournament_table_seats_position_chk CHECK (finish_position BETWEEN 1 AND 4),
  CONSTRAINT tournament_table_seats_unique_seat UNIQUE (tournament_table_id, tournament_participant_id)
);

CREATE INDEX tournament_table_seats_table_id_idx ON tournament_table_seats (tournament_table_id);
CREATE INDEX tournament_table_seats_participant_id_idx ON tournament_table_seats (tournament_participant_id);

-- Deny-all RLS on every new table, same as the rest of the public schema
-- (Supabase exposes it via PostgREST with an anon key; the backend accesses it
-- through a direct Postgres connection instead, see 00014's identical rationale).
ALTER TABLE tournaments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_rounds ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_tables ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_table_seats ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tournament_table_seats DISABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_tables DISABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_rounds DISABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_participants DISABLE ROW LEVEL SECURITY;
ALTER TABLE tournaments DISABLE ROW LEVEL SECURITY;

DROP TABLE tournament_table_seats;
DROP TABLE tournament_tables;
DROP TABLE tournament_rounds;
DROP TABLE tournament_participants;
DROP TABLE tournaments;

-- +goose StatementEnd
