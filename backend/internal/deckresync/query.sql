-- name: CreateResyncJob :one
INSERT INTO deck_resync_jobs (user_id, total_decks, status)
VALUES ($1, $2, 'in_progress')
RETURNING *;

-- name: GetResyncJob :one
SELECT * FROM deck_resync_jobs WHERE id = $1 LIMIT 1;

-- name: RecordResyncJobDeckResult :one
UPDATE deck_resync_jobs
SET updated_count = updated_count + sqlc.arg(updated_delta)::int,
    failed_count = failed_count + sqlc.arg(failed_delta)::int,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: FinishResyncJob :one
UPDATE deck_resync_jobs
SET status = sqlc.arg(status), error_message = sqlc.narg(error_message), finished_at = now(), updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ReapStaleResyncJobs :many
UPDATE deck_resync_jobs
SET status = 'failed',
    error_message = 'Interrupted by a server restart before finishing; please retry the resync.',
    finished_at = now(),
    updated_at = now()
WHERE status IN ('pending', 'in_progress')
RETURNING *;
