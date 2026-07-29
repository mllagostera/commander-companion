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

-- name: GetPlaygroupMember :one
SELECT * FROM playgroup_members WHERE playgroup_id = $1 AND user_id = $2 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: ListDecksByUserID :many
-- Decks de un usuario, para el picker de "con qué deck juega" un compañero de
-- grupo en un proxy-join (ver ADR-0013). La autorización (¿comparte el caller un
-- playgroup con este usuario?) la hace el service, no esta query.
SELECT * FROM decks WHERE user_id = $1 ORDER BY created_at DESC;
