<script setup lang="ts">
import type { DeckTraitFilter, ManaColor } from '~/types/api'

// Bracket and color-identity filter for the deck list and the games history.
// Toggle buttons (aria-pressed) rather than checkboxes: each one is a single
// visible chip, the same way the rest of the app draws its choices. Colorless
// is exclusive with the five colors -- a colorless deck has none of them.
const model = defineModel<DeckTraitFilter>({ required: true })

const { t } = useI18n()

const colorModeOptions = computed(() => [
  { value: 'exact', label: t('deckTraits.filter.mode.exact') },
  { value: 'includes', label: t('deckTraits.filter.mode.includes') },
  { value: 'within', label: t('deckTraits.filter.mode.within') },
])

function toggleBracket(bracket: number) {
  const brackets = model.value.brackets.includes(bracket)
    ? model.value.brackets.filter((b) => b !== bracket)
    : [...model.value.brackets, bracket]
  model.value = { ...model.value, brackets }
}

function toggleColor(color: ManaColor) {
  const colors = model.value.colors.includes(color)
    ? model.value.colors.filter((c) => c !== color)
    : sortWubrg([...model.value.colors, color])
  model.value = { ...model.value, colors, colorless: false }
}

function toggleColorless() {
  model.value = { ...model.value, colors: [], colorless: !model.value.colorless }
}

function clear() {
  model.value = emptyDeckTraitFilter()
}

const chipClass = 'inline-flex h-8 min-w-8 items-center justify-center rounded-full border px-2.5 text-[12px] font-semibold transition-colors'

function chipStyle(active: boolean): string {
  return active
    ? 'background: rgba(139,92,246,0.18); border-color: var(--accent-link); color: var(--text);'
    : 'border-color: var(--input-border); color: var(--text-muted);'
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-x-4 gap-y-2.5">
    <div role="group" :aria-label="$t('deckTraits.filter.bracketLabel')" class="flex flex-wrap items-center gap-1.5">
      <span class="mr-0.5 text-[11px] uppercase tracking-wide" style="color: var(--text-dim);" aria-hidden="true">
        {{ $t('deckTraits.filter.bracketLabel') }}
      </span>
      <button
        v-for="bracket in BRACKETS"
        :key="bracket"
        type="button"
        :class="chipClass"
        :style="chipStyle(model.brackets.includes(bracket))"
        :aria-pressed="model.brackets.includes(bracket)"
        :aria-label="$t('deckTraits.bracket', { n: bracket })"
        @click="toggleBracket(bracket)"
      >
        {{ bracket }}
      </button>
    </div>

    <div role="group" :aria-label="$t('deckTraits.filter.colorsLabel')" class="flex flex-wrap items-center gap-1.5">
      <span class="mr-0.5 text-[11px] uppercase tracking-wide" style="color: var(--text-dim);" aria-hidden="true">
        {{ $t('deckTraits.filter.colorsLabel') }}
      </span>
      <button
        v-for="color in MANA_COLORS"
        :key="color"
        type="button"
        :class="chipClass"
        class="!px-1.5"
        :style="chipStyle(model.colors.includes(color))"
        :aria-pressed="model.colors.includes(color)"
        :aria-label="$t(`deckTraits.colors.${color}`)"
        @click="toggleColor(color)"
      >
        <ManaPips :colors="[color]" size="md" aria-hidden="true" />
      </button>
      <button
        type="button"
        :class="chipClass"
        class="!px-1.5"
        :style="chipStyle(model.colorless)"
        :aria-pressed="model.colorless"
        :aria-label="$t('deckTraits.colors.C')"
        @click="toggleColorless"
      >
        <ManaPips :colors="[]" size="md" aria-hidden="true" />
      </button>
    </div>

    <SortSelect
      v-if="model.colors.length"
      :model-value="model.colorMode"
      :options="colorModeOptions"
      :select-label="$t('deckTraits.filter.mode.ariaLabel')"
      @update:model-value="(v) => (model = { ...model, colorMode: v as DeckTraitFilter['colorMode'] })"
    />

    <button
      v-if="isDeckTraitFilterActive(model)"
      type="button"
      class="py-1 text-[12px] underline"
      style="color: var(--accent-link);"
      @click="clear"
    >
      {{ $t('deckTraits.filter.clear') }}
    </button>
  </div>
</template>
