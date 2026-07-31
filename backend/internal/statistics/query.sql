-- name: GetUserStatistics :one
SELECT * FROM user_statistics_summary
WHERE user_id = $1 LIMIT 1;

-- name: GetDeckStatistics :one
SELECT * FROM deck_statistics_summary
WHERE deck_id = $1 LIMIT 1;

-- name: GetDeckByID :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;

-- name: ListGamePlayersForGame :many
SELECT * FROM game_players WHERE game_id = $1;

-- name: ListGameActionsForGame :many
SELECT * FROM game_actions WHERE game_id = $1 ORDER BY created_at ASC;

-- name: UpsertUserStatistics :exec
INSERT INTO user_statistics_summary (
  user_id, games_played, games_won, total_damage_dealt, total_commander_damage_dealt, total_eliminations
)
VALUES (sqlc.arg(user_id), sqlc.arg(games_played), sqlc.arg(games_won), sqlc.arg(total_damage_dealt),
        sqlc.arg(total_commander_damage_dealt), sqlc.arg(total_eliminations))
ON CONFLICT (user_id) DO UPDATE SET
  games_played = user_statistics_summary.games_played + EXCLUDED.games_played,
  games_won = user_statistics_summary.games_won + EXCLUDED.games_won,
  total_damage_dealt = user_statistics_summary.total_damage_dealt + EXCLUDED.total_damage_dealt,
  total_commander_damage_dealt = user_statistics_summary.total_commander_damage_dealt + EXCLUDED.total_commander_damage_dealt,
  total_eliminations = user_statistics_summary.total_eliminations + EXCLUDED.total_eliminations,
  last_recalculated_at = now();

-- name: UpsertDeckStatistics :exec
INSERT INTO deck_statistics_summary (
  deck_id, games_played, games_won, highest_life_total_achieved, total_commander_damage_dealt
)
VALUES (sqlc.arg(deck_id), sqlc.arg(games_played), sqlc.arg(games_won), sqlc.arg(highest_life_total_achieved),
        sqlc.arg(total_commander_damage_dealt))
ON CONFLICT (deck_id) DO UPDATE SET
  games_played = deck_statistics_summary.games_played + EXCLUDED.games_played,
  games_won = deck_statistics_summary.games_won + EXCLUDED.games_won,
  highest_life_total_achieved = GREATEST(deck_statistics_summary.highest_life_total_achieved, EXCLUDED.highest_life_total_achieved),
  total_commander_damage_dealt = deck_statistics_summary.total_commander_damage_dealt + EXCLUDED.total_commander_damage_dealt,
  last_recalculated_at = now();

-- name: CountFinishedGamesForPlaygroup :one
SELECT COUNT(*) FROM games WHERE playgroup_id = $1 AND status = 'finished';

-- name: ListPlaygroupMemberGameStats :many
SELECT
  gp.user_id,
  COUNT(DISTINCT gp.game_id)::int AS games_played,
  COUNT(DISTINCT winner.id)::int AS games_won
FROM game_players gp
JOIN games g ON g.id = gp.game_id
LEFT JOIN (
  -- The winner of a game is the only player still alive at the end; the
  -- alive_count = 1 filter selects that row without needing MIN()/MAX()
  -- over uuid (Postgres has no ordering operator for that type).
  SELECT id, game_id
  FROM (
    SELECT id, game_id, COUNT(*) OVER (PARTITION BY game_id) AS alive_count
    FROM game_players
    WHERE NOT is_eliminated
  ) alive
  WHERE alive_count = 1
) winner ON winner.game_id = gp.game_id AND winner.id = gp.id
WHERE g.playgroup_id = $1 AND g.status = 'finished'
GROUP BY gp.user_id;
