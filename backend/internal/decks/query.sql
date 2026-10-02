-- name: CreateDeck :one
INSERT INTO decks (
  user_id, name, commander, moxfield_id, image_url,
  bracket, color_identity, bracket_overridden, color_identity_overridden
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetDeck :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;

-- name: GetDeckByMoxfieldID :one
-- Resolves a user's already-imported deck from its public Moxfield ID
-- (see internal/sync). The same Moxfield deck can be imported by several
-- users, hence the filter by user_id.
SELECT * FROM decks
WHERE user_id = $1 AND moxfield_id = $2
ORDER BY created_at ASC
LIMIT 1;

-- name: ListDecksPage :many
-- Keyset pagination over (created_at, id) DESC. With cursor_created_at NULL
-- it returns the first page; with a cursor, the rows strictly after it in
-- list order. See internal/common/pagination.go.
--
-- brackets/colors narrow the list (NULL = no filter, see
-- common.DeckTraitFilter); a deck whose value is unknown never matches.
-- "exact" is containment both ways, so it doesn't depend on letter order.
SELECT * FROM decks
WHERE user_id = sqlc.arg('user_id')
  AND (sqlc.narg('brackets')::smallint[] IS NULL OR bracket = ANY(sqlc.narg('brackets')::smallint[]))
  AND (
    sqlc.narg('colors')::text[] IS NULL
    OR CASE sqlc.arg('color_mode')::text
      WHEN 'includes' THEN color_identity @> sqlc.narg('colors')::text[]
      WHEN 'within' THEN color_identity <@ sqlc.narg('colors')::text[]
      ELSE color_identity @> sqlc.narg('colors')::text[] AND color_identity <@ sqlc.narg('colors')::text[]
    END
  )
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: UpdateDeckFromMoxfield :one
-- Re-syncs name, commander, image, bracket and color identity for an
-- already-imported deck with what Moxfield returns today (see internal/sync).
-- A bracket/color identity set by hand (the *_overridden flags) is kept.
-- updated_at marks the last successful sync.
UPDATE decks
SET name = $2, commander = $3, image_url = $4,
    bracket = CASE WHEN bracket_overridden THEN bracket ELSE sqlc.narg('bracket')::smallint END,
    color_identity = CASE WHEN color_identity_overridden THEN color_identity ELSE sqlc.narg('color_identity')::text[] END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateDeckTraits :one
-- Writes a deck's bracket and color identity with their override flags as
-- decks.Service.UpdateDeck resolved them. Doesn't touch updated_at: that one
-- means "last Moxfield sync".
UPDATE decks
SET bracket = $2, color_identity = $3, bracket_overridden = $4, color_identity_overridden = $5
WHERE id = $1
RETURNING *;

-- name: BackfillSeatDeckTraits :exec
-- Copies the deck's current bracket and color identity onto its past seats
-- that had none when the game was played, and flags them as backfilled. A
-- backfilled seat keeps following the deck on every later change (its value
-- was a guess, and a mistake typed in must stay correctable); a seat that got
-- its value when it sat down keeps it: that's what was actually played.
-- Each field is handled on its own, since a seat can have one and not the other.
-- (On the right-hand side of SET, the columns read their pre-update values.)
UPDATE game_players
SET deck_bracket = CASE WHEN deck_bracket IS NULL OR deck_bracket_backfilled
                        THEN sqlc.narg('bracket')::smallint ELSE deck_bracket END,
    deck_bracket_backfilled = deck_bracket IS NULL OR deck_bracket_backfilled,
    deck_color_identity = CASE WHEN deck_color_identity IS NULL OR deck_color_identity_backfilled
                               THEN sqlc.narg('color_identity')::text[] ELSE deck_color_identity END,
    deck_color_identity_backfilled = deck_color_identity IS NULL OR deck_color_identity_backfilled
WHERE deck_id = sqlc.arg('deck_id')
  AND (deck_bracket IS NULL OR deck_bracket_backfilled
       OR deck_color_identity IS NULL OR deck_color_identity_backfilled);

-- name: DeleteDeck :exec
DELETE FROM decks WHERE id = $1;
