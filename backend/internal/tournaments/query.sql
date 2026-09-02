-- name: CreateTournament :one
INSERT INTO tournaments (organizer_id, name, target_players, join_code)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTournament :one
SELECT * FROM tournaments WHERE id = $1 LIMIT 1;

-- name: GetTournamentByJoinCode :one
SELECT * FROM tournaments WHERE join_code = $1 LIMIT 1;

-- name: ListTournamentsForUserPage :many
-- Tournaments the user organizes OR participates in, keyset-paginated over
-- (created_at, id) DESC, same shape as playgroups.ListPlaygroupsForUserPage.
SELECT DISTINCT t.* FROM tournaments t
LEFT JOIN tournament_participants tp
  ON tp.tournament_id = t.id AND tp.user_id = sqlc.arg('user_id')::uuid
WHERE (t.organizer_id = sqlc.arg('user_id')::uuid OR tp.id IS NOT NULL)
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (t.created_at, t.id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg('page_limit');

-- name: StartTournament :one
UPDATE tournaments
SET status = 'in_progress', round_count = sqlc.arg('round_count'), current_round = 1, started_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: AdvanceTournamentRound :one
UPDATE tournaments SET current_round = current_round + 1 WHERE id = $1 RETURNING *;

-- name: FinishTournament :one
UPDATE tournaments SET status = 'finished', finished_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteTournament :exec
DELETE FROM tournaments WHERE id = $1;

-- name: DeleteParticipantsForTournament :exec
-- Only needed to delete a tournament still in 'registration': rounds/tables/seats
-- don't exist yet at that point (StartTournament is what creates the first ones),
-- so participants are the only rows referencing it. See service.DeleteTournament.
DELETE FROM tournament_participants WHERE tournament_id = $1;

-- name: CreateParticipant :one
INSERT INTO tournament_participants (tournament_id, user_id, guest_name, deck_id, commander_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetParticipantByUser :one
SELECT tp.id, tp.tournament_id, tp.user_id, tp.guest_name, tp.deck_id, tp.commander_name,
       tp.points, tp.created_at,
       u.username AS username, d.name AS deck_name, d.commander AS deck_commander
FROM tournament_participants tp
LEFT JOIN users u ON u.id = tp.user_id
LEFT JOIN decks d ON d.id = tp.deck_id
WHERE tp.tournament_id = $1 AND tp.user_id = $2
LIMIT 1;

-- name: GetParticipant :one
SELECT * FROM tournament_participants WHERE id = $1 LIMIT 1;

-- name: ListParticipantsForTournament :many
-- Standings order: most points first, registration order breaks ties (see
-- pairing.go's doc for why no fancier tiebreaker is computed).
SELECT tp.id, tp.tournament_id, tp.user_id, tp.guest_name, tp.deck_id, tp.commander_name,
       tp.points, tp.created_at,
       u.username AS username, d.name AS deck_name, d.commander AS deck_commander
FROM tournament_participants tp
LEFT JOIN users u ON u.id = tp.user_id
LEFT JOIN decks d ON d.id = tp.deck_id
WHERE tp.tournament_id = $1
ORDER BY tp.points DESC, tp.created_at ASC;

-- name: CountParticipants :one
SELECT COUNT(*)::int FROM tournament_participants WHERE tournament_id = $1;

-- name: AddParticipantPoints :one
UPDATE tournament_participants SET points = points + sqlc.arg('delta')::int WHERE id = sqlc.arg('id')
RETURNING *;

-- name: CreateRound :one
INSERT INTO tournament_rounds (tournament_id, round_number)
VALUES ($1, $2)
RETURNING *;

-- name: GetRoundByNumber :one
SELECT * FROM tournament_rounds WHERE tournament_id = $1 AND round_number = $2 LIMIT 1;

-- name: FinishRound :one
UPDATE tournament_rounds SET status = 'finished', finished_at = now() WHERE id = $1 RETURNING *;

-- name: CreateTable :one
INSERT INTO tournament_tables (tournament_round_id, table_number)
VALUES ($1, $2)
RETURNING *;

-- name: GetTable :one
SELECT * FROM tournament_tables WHERE id = $1 LIMIT 1;

-- name: GetTableForParticipantInRound :one
SELECT tt.* FROM tournament_tables tt
JOIN tournament_table_seats tts ON tts.tournament_table_id = tt.id
WHERE tt.tournament_round_id = $1 AND tts.tournament_participant_id = $2
LIMIT 1;

-- name: GetRound :one
SELECT * FROM tournament_rounds WHERE id = $1 LIMIT 1;

-- name: CreateTableSeat :one
INSERT INTO tournament_table_seats (tournament_table_id, tournament_participant_id)
VALUES ($1, $2)
RETURNING *;

-- name: RecordSeatResult :one
UPDATE tournament_table_seats
SET finish_position = sqlc.arg('finish_position'), points_awarded = sqlc.arg('points_awarded')
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListSeatsForRound :many
-- Enriched fetch of every table+seat in a round (table number, seat/participant
-- identity, and result if recorded) in one query, same batched-enrichment shape
-- as statistics.ListPlayersForGames -- grouped by table_number in Go.
SELECT
  tt.id AS table_id, tt.table_number,
  tts.id AS seat_id, tts.tournament_participant_id, tts.finish_position, tts.points_awarded,
  tp.user_id, tp.guest_name, tp.commander_name AS participant_commander_name,
  u.username, d.name AS deck_name, d.commander AS deck_commander
FROM tournament_tables tt
JOIN tournament_table_seats tts ON tts.tournament_table_id = tt.id
JOIN tournament_participants tp ON tp.id = tts.tournament_participant_id
LEFT JOIN users u ON u.id = tp.user_id
LEFT JOIN decks d ON d.id = tp.deck_id
WHERE tt.tournament_round_id = $1
ORDER BY tt.table_number, tp.points DESC;

-- name: ListSeatsForTable :many
SELECT tts.* FROM tournament_table_seats tts WHERE tts.tournament_table_id = $1;

-- name: ListEnrichedSeatsForTable :many
-- Same enrichment as ListSeatsForRound, scoped to a single table (used to
-- render the response right after RecordTableResult, without re-fetching the
-- whole round).
SELECT
  tts.id AS seat_id, tts.tournament_participant_id, tts.finish_position, tts.points_awarded,
  tp.user_id, tp.guest_name, tp.commander_name AS participant_commander_name,
  u.username, d.name AS deck_name, d.commander AS deck_commander
FROM tournament_table_seats tts
JOIN tournament_participants tp ON tp.id = tts.tournament_participant_id
LEFT JOIN users u ON u.id = tp.user_id
LEFT JOIN decks d ON d.id = tp.deck_id
WHERE tts.tournament_table_id = $1
ORDER BY tp.points DESC;

-- name: CountUnrecordedSeatsInRound :one
SELECT COUNT(*)::int FROM tournament_table_seats tts
JOIN tournament_tables tt ON tt.id = tts.tournament_table_id
WHERE tt.tournament_round_id = $1 AND tts.finish_position IS NULL;

-- name: ListPastSeatsForTournament :many
-- Every (table_id, participant_id) pair across every round already played in this
-- tournament, for pairing.go to compute who's already played whom.
SELECT tts.tournament_table_id, tts.tournament_participant_id
FROM tournament_table_seats tts
JOIN tournament_tables tt ON tt.id = tts.tournament_table_id
JOIN tournament_rounds tr ON tr.id = tt.tournament_round_id
WHERE tr.tournament_id = $1;
