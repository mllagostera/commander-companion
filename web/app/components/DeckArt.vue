<script setup lang="ts">
import type { Deck } from '~/types/api'

withDefaults(defineProps<{ deck: Deck; aspectRatio?: string; rounded?: string }>(), {
  aspectRatio: '1',
  rounded: 'rounded-[18px]',
})
</script>

<template>
  <div
    class="relative overflow-hidden border"
    :class="rounded"
    :style="{ aspectRatio, borderColor: 'var(--card-border)' }"
  >
    <div v-if="!deck.image_url" class="cc-deck-shimmer" />
    <img
      v-if="deck.image_url"
      :src="deck.image_url"
      :alt="deck.commander"
      class="absolute inset-0 h-full w-full object-cover"
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
