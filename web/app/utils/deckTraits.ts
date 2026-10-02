import type { DeckTraitFilter, ManaColor } from '~/types/api'

export function emptyDeckTraitFilter(): DeckTraitFilter {
  return { brackets: [], colors: [], colorless: false, colorMode: 'exact' }
}

export function isDeckTraitFilterActive(filter: DeckTraitFilter): boolean {
  return filter.brackets.length > 0 || filter.colors.length > 0 || filter.colorless
}

/**
 * The filter as `GET /decks` / `GET /statistics/games` query params, leaving
 * out what doesn't filter so an inactive filter adds nothing to the URL.
 */
export function deckTraitFilterQuery(filter: DeckTraitFilter): Record<string, string> {
  const query: Record<string, string> = {}
  if (filter.brackets.length) query.bracket = [...filter.brackets].sort().join(',')
  if (filter.colorless) {
    query.colors = 'C'
  } else if (filter.colors.length) {
    query.colors = sortWubrg(filter.colors).join('')
    query.color_mode = filter.colorMode
  }
  return query
}

export function sortWubrg(colors: readonly ManaColor[]): ManaColor[] {
  return MANA_COLORS.filter((c) => colors.includes(c))
}

// The array constants go last on purpose: Nuxt's auto-import scanner (mlly)
// silently drops an exported function that follows an exported array literal,
// so placed first they hid emptyDeckTraitFilter from auto-import.

/** The five colors in WUBRG order, the order the API returns them in. */
export const MANA_COLORS: readonly ManaColor[] = ['W', 'U', 'B', 'R', 'G']

/** The Commander brackets, 1 to 5. */
export const BRACKETS: readonly number[] = [1, 2, 3, 4, 5]
