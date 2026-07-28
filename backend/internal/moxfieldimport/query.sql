-- name: CreateImportJob :one
INSERT INTO moxfield_import_jobs (user_id, moxfield_username)
VALUES ($1, $2)
RETURNING *;

-- name: GetImportJob :one
SELECT * FROM moxfield_import_jobs WHERE id = $1 LIMIT 1;

-- name: SetImportJobInProgress :one
UPDATE moxfield_import_jobs
SET status = 'in_progress', total_decks = sqlc.arg(total_decks), updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RecordImportJobDeckResult :one
UPDATE moxfield_import_jobs
SET imported_count = imported_count + sqlc.arg(imported_delta)::int,
    failed_count = failed_count + sqlc.arg(failed_delta)::int,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: FinishImportJob :one
UPDATE moxfield_import_jobs
SET status = sqlc.arg(status), error_message = sqlc.narg(error_message), finished_at = now(), updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
