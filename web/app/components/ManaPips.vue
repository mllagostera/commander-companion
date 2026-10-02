<script setup lang="ts">
import type { ManaColor } from '~/types/api'

// A color identity as a row of mana-colored discs, one per color in WUBRG
// order. `[]` is colorless (a single C disc); null is unknown and draws nothing,
// so a deck nobody has filled in doesn't look colorless.
const props = withDefaults(
  defineProps<{
    colors: ManaColor[] | null
    size?: 'sm' | 'md'
  }>(),
  { size: 'sm' },
)

const { t } = useI18n()

const pips = computed<(ManaColor | 'C')[]>(() => {
  if (props.colors === null) return []
  return props.colors.length ? sortWubrg(props.colors) : ['C']
})

const label = computed(() => pips.value.map((c) => t(`deckTraits.colors.${c}`)).join(', '))
</script>

<template>
  <span v-if="pips.length" role="img" :aria-label="label" :title="label" class="inline-flex items-center gap-0.5">
    <span
      v-for="color in pips"
      :key="color"
      aria-hidden="true"
      class="inline-flex items-center justify-center rounded-full font-bold leading-none"
      :class="size === 'md' ? 'h-5 w-5 text-[10px]' : 'h-4 w-4 text-[9px]'"
      :style="{ background: `var(--mana-${color.toLowerCase()})`, color: 'var(--mana-ink)' }"
    >{{ color }}</span>
  </span>
</template>
