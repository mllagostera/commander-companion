-- name: GetUserStatistics :one
SELECT * FROM user_statistics_summary
WHERE user_id = $1 LIMIT 1;

-- name: GetDeckStatistics :one
SELECT * FROM deck_statistics_summary
WHERE deck_id = $1 LIMIT 1;
