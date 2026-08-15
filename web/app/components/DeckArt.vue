<script setup lang="ts">
import type { Deck } from '~/types/api'

const props = withDefaults(
  defineProps<{
    deck: Deck
    aspectRatio?: string
    rounded?: string
    imagePosition?: 'center' | 'right'
    /**
     * Fill the parent instead of sizing itself: drops the aspect ratio and the
     * border so the art can sit behind a card's content (the dashboard's best-deck
     * spotlight). The parent owns the size and the rounding in that mode.
     */
    fill?: boolean
  }>(),
  {
    aspectRatio: '1',
    rounded: 'rounded-[var(--radius-md)]',
    imagePosition: 'center',
    fill: false,
  },
)
</script>

<template>
  <!--
    In fill mode the root positions itself absolutely: it can't just receive
    `absolute` from the parent's class, because Tailwind emits `.relative`
    after `.absolute` in the stylesheet, so the root's own `relative` would win
    the tie on equal specificity regardless of class order.
  -->
  <div
    class="overflow-hidden"
    :class="[rounded, fill ? 'absolute inset-0 h-full w-full' : 'relative border']"
    :style="fill ? {} : { aspectRatio, borderColor: 'var(--card-border)' }"
  >
    <div v-if="!deck.image_url" class="cc-deck-shimmer" />
    <img
      v-if="deck.image_url"
      :src="deck.image_url"
      :alt="deck.commander"
      class="absolute inset-0 h-full w-full object-cover"
      :style="{ objectPosition: props.imagePosition === 'right' ? '85% center' : 'center' }"
    >
    <div
      v-else
      class="absolute inset-0 flex items-center justify-center text-2xl font-bold"
      style="background: linear-gradient(160deg, rgba(139,92,246,0.35), rgba(168,85,247,0.15)); color: var(--text-muted);"
    >
      {{ deck.commander?.[0]?.toUpperCase() ?? '?' }}
    </div>
  </div>
</template>
