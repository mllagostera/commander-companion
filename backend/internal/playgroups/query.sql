-- name: CreatePlaygroup :one
INSERT INTO playgroups (name)
VALUES ($1)
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
SELECT * FROM playgroup_members WHERE playgroup_id = $1;
