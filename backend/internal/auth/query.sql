-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1 LIMIT 1;

-- name: RotateRefreshToken :execrows
-- Conditional on the token still being active: when two refreshes with the same
-- token race, only the first one records its successor (0 rows for the other).
UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2
WHERE id = $1 AND revoked_at IS NULL;

-- name: HasActiveSuccessorWithinGrace :one
-- Whether a rotated token is being presented again within the reuse grace window
-- (seconds, $2) while the token it was rotated into is still active — the shape of
-- two concurrent refreshes by one client, not of a replayed stolen token. Compared
-- in SQL so revoked_at (set with now()) and the window share one clock. See ADR-0028.
SELECT EXISTS (
  SELECT 1 FROM refresh_tokens parent
  JOIN refresh_tokens successor ON successor.id = parent.replaced_by
  WHERE parent.id = $1
    AND parent.revoked_at > now() - make_interval(secs => sqlc.arg(grace_seconds)::float8)
    AND successor.revoked_at IS NULL
) AS within_grace;

-- name: RevokeRefreshTokenByHash :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE token_hash = $1;

-- name: RevokeAllRefreshTokensForUser :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;
