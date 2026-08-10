-- name: CreatePlaygroup :one
INSERT INTO playgroups (name)
VALUES ($1)
RETURNING *;

-- name: UpdatePlaygroupName :one
UPDATE playgroups SET name = $2
WHERE id = $1
RETURNING *;

-- name: GetPlaygroup :one
SELECT * FROM playgroups WHERE id = $1 LIMIT 1;

-- name: ListPlaygroups :many
SELECT * FROM playgroups ORDER BY created_at DESC;

-- name: AddPlaygroupMember :one
INSERT INTO playgroup_members (playgroup_id, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: ListPlaygroupMembers :many
SELECT pm.playgroup_id, pm.user_id, pm.joined_at, u.username
FROM playgroup_members pm
JOIN users u ON u.id = pm.user_id
WHERE pm.playgroup_id = $1;

-- name: ListPlaygroupsForUser :many
SELECT p.* FROM playgroups p
JOIN playgroup_members pm ON pm.playgroup_id = p.id
WHERE pm.user_id = $1
ORDER BY p.created_at DESC;

-- name: ListPlaygroupsForUserPage :many
-- Keyset pagination over (created_at, id) DESC, scoped to playgroups the user is
-- a member of (same shape as games.ListGamesPage). With cursor_created_at NULL
-- it returns the first page; with a cursor, the rows strictly after it in list
-- order. See internal/common/pagination.go.
SELECT p.* FROM playgroups p
JOIN playgroup_members pm ON pm.playgroup_id = p.id
WHERE pm.user_id = sqlc.arg('user_id')::uuid
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (p.created_at, p.id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg('page_limit');

-- name: GetPlaygroupMember :one
SELECT * FROM playgroup_members WHERE playgroup_id = $1 AND user_id = $2 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: ListDecksByUserID :many
-- A user's decks, for the "which deck are they playing" picker when proxy-joining
-- a fellow group member (see ADR-0013). Authorization (does the caller share a
-- playgroup with this user?) is done by the service, not this query.
SELECT * FROM decks WHERE user_id = $1 ORDER BY created_at DESC;
