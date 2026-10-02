<script setup lang="ts">
import type { ManaColor } from '~/types/api'

// Picks a deck's bracket and color identity, for the create and edit forms.
// Both can be left unknown ("?"), which is different from colorless: a deck
// nobody filled in shouldn't show up when filtering for colorless decks.
const bracket = defineModel<number | null>('bracket', { required: true })
const colors = defineModel<ManaColor[] | null>('colors', { required: true })

defineProps<{
  /** Prefix for the group ids, so two editors on a page don't collide. */
  idPrefix: string
}>()

function toggleColor(color: ManaColor) {
  const current = colors.value ?? []
  colors.value = current.includes(color) ? current.filter((c) => c !== color) : sortWubrg([...current, color])
}

const chipClass = 'inline-flex h-8 min-w-8 items-center justify-center rounded-full border px-2.5 text-[12px] font-semibold transition-colors'

function chipStyle(active: boolean): string {
  return active
    ? 'background: rgba(139,92,246,0.18); border-color: var(--accent-link); color: var(--text);'
    : 'border-color: var(--input-border); color: var(--text-muted);'
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-col gap-1.5">
      <span :id="`${idPrefix}-bracket`" class="px-1 text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
        {{ $t('deckTraits.edit.bracketLabel') }}
      </span>
      <div role="group" :aria-labelledby="`${idPrefix}-bracket`" class="flex flex-wrap gap-1.5">
        <button
          type="button"
          :class="chipClass"
          :style="chipStyle(bracket === null)"
          :aria-pressed="bracket === null"
          :aria-label="$t('deckTraits.edit.unknown')"
          @click="bracket = null"
        >
          ?
        </button>
        <button
          v-for="b in BRACKETS"
          :key="b"
          type="button"
          :class="chipClass"
          :style="chipStyle(bracket === b)"
          :aria-pressed="bracket === b"
          :aria-label="$t('deckTraits.bracket', { n: b })"
          @click="bracket = b"
        >
          {{ b }}
        </button>
      </div>
    </div>

    <div class="flex flex-col gap-1.5">
      <span :id="`${idPrefix}-colors`" class="px-1 text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
        {{ $t('deckTraits.edit.colorsLabel') }}
      </span>
      <div role="group" :aria-labelledby="`${idPrefix}-colors`" class="flex flex-wrap gap-1.5">
        <button
          type="button"
          :class="chipClass"
          :style="chipStyle(colors === null)"
          :aria-pressed="colors === null"
          :aria-label="$t('deckTraits.edit.unknown')"
          @click="colors = null"
        >
          ?
        </button>
        <button
          v-for="color in MANA_COLORS"
          :key="color"
          type="button"
          :class="chipClass"
          class="!px-1.5"
          :style="chipStyle(!!colors?.includes(color))"
          :aria-pressed="!!colors?.includes(color)"
          :aria-label="$t(`deckTraits.colors.${color}`)"
          @click="toggleColor(color)"
        >
          <ManaPips :colors="[color]" size="md" aria-hidden="true" />
        </button>
        <button
          type="button"
          :class="chipClass"
          class="!px-1.5"
          :style="chipStyle(colors !== null && colors.length === 0)"
          :aria-pressed="colors !== null && colors.length === 0"
          :aria-label="$t('deckTraits.colors.C')"
          @click="colors = []"
        >
          <ManaPips :colors="[]" size="md" aria-hidden="true" />
        </button>
      </div>
    </div>
  </div>
</template>
