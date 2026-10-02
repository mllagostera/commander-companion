<script setup lang="ts">
import type { ManaColor } from '~/types/api'

// A deck's bracket badge and color pips side by side. `onArt` is for text laid
// over deck art (the deck grid), where the page's theme colors don't apply.
// Draws nothing when both are unknown.
withDefaults(
  defineProps<{
    bracket: number | null
    colors: ManaColor[] | null
    onArt?: boolean
  }>(),
  { onArt: false },
)
</script>

<template>
  <span v-if="bracket !== null || colors !== null" class="inline-flex items-center gap-1.5">
    <span
      v-if="bracket !== null"
      class="rounded-full border px-1.5 py-px text-[10px] font-semibold"
      :class="onArt ? 'border-white/30 text-white/90' : ''"
      :style="onArt ? '' : 'border-color: var(--card-border); color: var(--text-muted);'"
      :title="$t('deckTraits.bracket', { n: bracket })"
    >
      <span aria-hidden="true">{{ $t('deckTraits.bracketShort', { n: bracket }) }}</span>
      <span class="sr-only">{{ $t('deckTraits.bracket', { n: bracket }) }}</span>
    </span>
    <ManaPips :colors="colors" />
  </span>
</template>
