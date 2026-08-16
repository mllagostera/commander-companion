<script setup lang="ts">
/**
 * Placeholder for a section that has nothing to show yet, with an action that
 * gets the user out of the empty state. Every dashboard section uses this
 * instead of a bare grey sentence: a brand-new account otherwise lands on a
 * page made entirely of "nothing here yet" text with nothing to click.
 *
 * The dashed border is what distinguishes it from a real content card at a
 * glance — the surface reads as "this will fill in", not as a broken card.
 */
defineProps<{ title: string, body?: string, ctaLabel?: string, ctaTo?: string }>()

// The action is a link when it navigates somewhere (ctaTo) and a button
// otherwise — some empty states resolve in place, e.g. by opening the deck
// import modal, and those emit `cta` instead.
defineEmits<{ cta: [] }>()
</script>

<template>
  <div
    class="flex flex-col items-start justify-center gap-3.5 rounded-[var(--radius-lg)] border border-dashed p-6"
    style="border-color: var(--input-border); background: var(--card-bg);"
  >
    <div>
      <p class="text-sm font-medium" style="color: var(--text);">{{ title }}</p>
      <p v-if="body" class="mt-1.5 text-[13px] leading-relaxed" style="color: var(--text-muted);">{{ body }}</p>
    </div>
    <NuxtLink
      v-if="ctaLabel && ctaTo"
      :to="ctaTo"
      class="rounded-full px-4 py-2 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.04]"
      style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
    >
      {{ ctaLabel }}
    </NuxtLink>
    <button
      v-else-if="ctaLabel"
      type="button"
      class="rounded-full px-4 py-2 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.04]"
      style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
      @click="$emit('cta')"
    >
      {{ ctaLabel }}
    </button>
  </div>
</template>
