-- name: CreateDeck :one
INSERT INTO decks (user_id, name, commander, moxfield_id, image_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDeck :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;

-- name: GetDeckByMoxfieldID :one
-- Resuelve el deck ya importado de un usuario a partir de su ID público de
-- Moxfield (ver internal/sync). Un mismo deck de Moxfield puede estar importado
-- por varios usuarios, por eso el filtro por user_id.
SELECT * FROM decks
WHERE user_id = $1 AND moxfield_id = $2
ORDER BY created_at ASC
LIMIT 1;

-- name: ListDecksPage :many
-- Paginación keyset sobre (created_at, id) DESC. Con cursor_created_at NULL
-- devuelve la primera página; con cursor, las filas estrictamente posteriores en
-- el orden de la lista. Ver internal/common/pagination.go.
SELECT * FROM decks
WHERE user_id = sqlc.arg('user_id')
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: UpdateDeckFromMoxfield :one
-- Re-sincroniza nombre, comandante e imagen de un deck ya importado con lo que
-- devuelve Moxfield hoy (ver internal/sync). updated_at marca el último sync exitoso.
UPDATE decks
SET name = $2, commander = $3, image_url = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDeck :exec
DELETE FROM decks WHERE id = $1;
