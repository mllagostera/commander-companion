-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: CreateFriendRequest :one
INSERT INTO friend_requests (requester_id, addressee_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetFriendRequestByID :one
SELECT * FROM friend_requests WHERE id = $1 LIMIT 1;

-- name: GetPendingRequestBetween :one
-- The pending request from requester_id to addressee_id, if any (same direction).
SELECT * FROM friend_requests
WHERE requester_id = $1 AND addressee_id = $2 AND status = 'pending'
LIMIT 1;

-- name: GetAcceptedFriendship :one
-- Whether user_a and user_b are already friends, regardless of who originally sent the request.
SELECT * FROM friend_requests
WHERE status = 'accepted'
  AND ((requester_id = sqlc.arg('user_a') AND addressee_id = sqlc.arg('user_b'))
    OR (requester_id = sqlc.arg('user_b') AND addressee_id = sqlc.arg('user_a')))
LIMIT 1;

-- name: RespondFriendRequest :one
-- Sets status to 'accepted'/'rejected'/'cancelled' and stamps responded_at. The
-- caller (service) is responsible for only passing one of those three values.
UPDATE friend_requests
SET status = sqlc.arg('status'), responded_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListIncomingFriendRequests :many
-- Pending requests addressed to user_id, with the requester's username.
SELECT fr.id, fr.requester_id, fr.addressee_id, fr.status, fr.created_at, fr.responded_at,
       u.username AS requester_username
FROM friend_requests fr
JOIN users u ON u.id = fr.requester_id
WHERE fr.addressee_id = $1 AND fr.status = 'pending'
ORDER BY fr.created_at DESC;

-- name: ListOutgoingFriendRequests :many
-- Pending requests sent by user_id, with the addressee's username.
SELECT fr.id, fr.requester_id, fr.addressee_id, fr.status, fr.created_at, fr.responded_at,
       u.username AS addressee_username
FROM friend_requests fr
JOIN users u ON u.id = fr.addressee_id
WHERE fr.requester_id = $1 AND fr.status = 'pending'
ORDER BY fr.created_at DESC;

-- name: ListFriends :many
-- Accepted friendships of user_id, resolved to the OTHER user in the row
-- regardless of who originally sent the request.
SELECT
  fr.id,
  (CASE WHEN fr.requester_id = sqlc.arg('user_id') THEN fr.addressee_id ELSE fr.requester_id END)::uuid AS friend_id,
  (CASE WHEN fr.requester_id = sqlc.arg('user_id') THEN au.username ELSE ru.username END)::varchar AS friend_username,
  fr.responded_at AS friends_since
FROM friend_requests fr
JOIN users ru ON ru.id = fr.requester_id
JOIN users au ON au.id = fr.addressee_id
WHERE fr.status = 'accepted'
  AND (fr.requester_id = sqlc.arg('user_id') OR fr.addressee_id = sqlc.arg('user_id'))
ORDER BY fr.created_at DESC;

-- name: DeleteFriendship :exec
-- Removes an accepted friendship between user_a and user_b, regardless of direction.
DELETE FROM friend_requests
WHERE status = 'accepted'
  AND ((requester_id = sqlc.arg('user_a') AND addressee_id = sqlc.arg('user_b'))
    OR (requester_id = sqlc.arg('user_b') AND addressee_id = sqlc.arg('user_a')));
