-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username varchar UNIQUE NOT NULL,
  email varchar UNIQUE NOT NULL,
  password_hash varchar NOT NULL,
  created_at timestamp DEFAULT (now()),
  updated_at timestamp
);

CREATE TABLE decks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid REFERENCES users(id),
  name varchar NOT NULL,
  commander varchar NOT NULL,
  moxfield_id varchar,
  created_at timestamp DEFAULT (now()),
  updated_at timestamp
);

CREATE TABLE playgroups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name varchar NOT NULL,
  created_at timestamp DEFAULT (now()),
  updated_at timestamp
);

CREATE TABLE playgroup_members (
  playgroup_id uuid REFERENCES playgroups(id),
  user_id uuid REFERENCES users(id),
  joined_at timestamp DEFAULT (now()),
  PRIMARY KEY (playgroup_id, user_id)
);

CREATE TABLE games (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  playgroup_id uuid REFERENCES playgroups(id),
  status varchar NOT NULL,
  started_at timestamp,
  finished_at timestamp,
  created_at timestamp DEFAULT (now())
);

CREATE TABLE game_players (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id uuid REFERENCES games(id),
  user_id uuid REFERENCES users(id),
  deck_id uuid REFERENCES decks(id),
  life_total int DEFAULT 40,
  poison_counters int DEFAULT 0,
  energy_counters int DEFAULT 0,
  experience_counters int DEFAULT 0,
  is_eliminated boolean DEFAULT false
);

CREATE TABLE game_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  game_id uuid REFERENCES games(id),
  actor_id uuid REFERENCES game_players(id),
  target_id uuid REFERENCES game_players(id),
  action_type varchar NOT NULL,
  payload jsonb,
  created_at timestamp DEFAULT (now())
);

CREATE TABLE user_statistics_summary (
  user_id uuid PRIMARY KEY REFERENCES users(id),
  games_played int DEFAULT 0,
  games_won int DEFAULT 0,
  total_damage_dealt int DEFAULT 0,
  total_commander_damage_dealt int DEFAULT 0,
  total_eliminations int DEFAULT 0,
  last_recalculated_at timestamp DEFAULT (now())
);

CREATE TABLE deck_statistics_summary (
  deck_id uuid PRIMARY KEY REFERENCES decks(id),
  games_played int DEFAULT 0,
  games_won int DEFAULT 0,
  highest_life_total_achieved int DEFAULT 0,
  total_commander_damage_dealt int DEFAULT 0,
  last_recalculated_at timestamp DEFAULT (now())
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE deck_statistics_summary;
DROP TABLE user_statistics_summary;
DROP TABLE game_actions;
DROP TABLE game_players;
DROP TABLE games;
DROP TABLE playgroup_members;
DROP TABLE playgroups;
DROP TABLE decks;
DROP TABLE users;
-- +goose StatementEnd
