/**
 * Contract between the Scryfall typeahead endpoint
 * (server/api/scryfall/commanders.get.ts) and its only consumer
 * (app/components/CommanderSearch.vue).
 *
 * It lives in shared/ rather than in app/types/api.ts because that file
 * mirrors the Go API's schemas from docs/api/openapi.yaml, and this shape is
 * neither: it never reaches the Go API. Only `name` and `image_url` do, as
 * two ordinary fields of `POST /decks`.
 */
export interface CommanderSuggestion {
  name: string
  /** e.g. "Legendary Creature — Phyrexian Angel Horror". Empty if Scryfall omits it. */
  type_line: string
  /** Scryfall's `art_crop`, or null for the rare card with no art. */
  image_url: string | null
}
