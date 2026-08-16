<script setup lang="ts">
import type { CommanderSuggestion } from '#shared/types/scryfall'

/**
 * Commander field for the "new deck" form: a text input that suggests real
 * cards from Scryfall as you type (see server/api/scryfall/commanders.get.ts
 * for why the lookup goes through Nitro).
 *
 * Free text is deliberately still accepted. The suggestions exist to save
 * typing and to attach the card's art, not to constrain the field — a
 * playgroup's houseruled or un-printed commander must remain writable, and
 * `POST /decks` takes commander as a plain string anyway.
 *
 * This is the ARIA 1.2 combobox pattern rather than SortSelect's listbox:
 * focus has to stay in the input while the user keeps typing, so the
 * highlighted option is pointed at with aria-activedescendant instead of
 * being focused for real.
 */
const props = defineProps<{
  modelValue: string
  inputId: string
  label: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** Fires only on an actual pick, carrying the art the deck should keep. */
  'select': [suggestion: CommanderSuggestion]
}>()

const DEBOUNCE_MS = 250
/** Matches MIN_QUERY_LENGTH in the endpoint; below it the API returns []. */
const MIN_QUERY_LENGTH = 2

const listboxId = useId()
const suggestions = ref<CommanderSuggestion[]>([])
const isOpen = ref(false)
const isLoading = ref(false)
const hasSearched = ref(false)
const activeIndex = ref(-1)
const rootRef = ref<HTMLElement | null>(null)

function optionId(index: number) {
  return `${listboxId}-option-${index}`
}

const activeDescendant = computed(() =>
  isOpen.value && activeIndex.value >= 0 ? optionId(activeIndex.value) : undefined,
)

function close() {
  isOpen.value = false
  activeIndex.value = -1
}

/**
 * Suppresses the request that a pick would otherwise trigger: choosing a
 * suggestion writes its full name back into the input, and the watcher below
 * can't tell that from the user typing that same text by hand.
 */
let skipNextSearch = false
let debounceHandle: ReturnType<typeof setTimeout> | undefined
/**
 * Guards against out-of-order responses. Without it, a slow request for "at"
 * landing after a fast one for "atraxa" would repopulate the list with
 * results for text the user has already moved past.
 */
let latestRequest = 0

async function search(query: string) {
  const requestId = ++latestRequest
  isLoading.value = true
  try {
    const results = await $fetch<CommanderSuggestion[]>('/api/scryfall/commanders', { query: { q: query } })
    if (requestId !== latestRequest) return
    suggestions.value = results
    activeIndex.value = -1
    isOpen.value = true
  } catch {
    // A failed lookup shouldn't block the form: the field still accepts
    // whatever the user typed, so the dropdown just stays empty.
    if (requestId !== latestRequest) return
    suggestions.value = []
    isOpen.value = true
  } finally {
    if (requestId === latestRequest) {
      isLoading.value = false
      hasSearched.value = true
    }
  }
}

watch(
  () => props.modelValue,
  (value) => {
    clearTimeout(debounceHandle)
    if (skipNextSearch) {
      skipNextSearch = false
      return
    }
    const query = value.trim()
    hasSearched.value = false
    if (query.length < MIN_QUERY_LENGTH) {
      suggestions.value = []
      close()
      return
    }
    debounceHandle = setTimeout(() => search(query), DEBOUNCE_MS)
  },
)

function choose(suggestion: CommanderSuggestion) {
  skipNextSearch = true
  emit('update:modelValue', suggestion.name)
  emit('select', suggestion)
  close()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (isOpen.value) {
      // Only swallow Escape when it closed the dropdown — otherwise it has to
      // keep bubbling to the modal, which closes on Escape too.
      event.stopPropagation()
      close()
    }
    return
  }
  if (!isOpen.value || !suggestions.value.length) return

  if (event.key === 'ArrowDown') {
    event.preventDefault()
    activeIndex.value = (activeIndex.value + 1) % suggestions.value.length
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    activeIndex.value = (activeIndex.value - 1 + suggestions.value.length) % suggestions.value.length
  } else if (event.key === 'Enter') {
    const active = suggestions.value[activeIndex.value]
    // With nothing highlighted, Enter belongs to the form: the typed text is
    // already a valid commander name.
    if (!active) return
    event.preventDefault()
    choose(active)
  }
}

function handleDocumentClick(event: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) close()
}

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => {
  document.removeEventListener('click', handleDocumentClick)
  clearTimeout(debounceHandle)
})
</script>

<template>
  <div ref="rootRef" class="relative">
    <input
      :id="inputId"
      :value="modelValue"
      type="text"
      role="combobox"
      autocomplete="off"
      aria-autocomplete="list"
      :aria-controls="listboxId"
      :aria-expanded="isOpen"
      :aria-activedescendant="activeDescendant"
      :aria-label="label"
      :placeholder="$t('decks.create.commanderPlaceholder')"
      class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
      style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      @keydown="handleKeydown"
    >

    <p v-if="isLoading" class="mt-1.5 px-4 text-[11px]" style="color: var(--text-dim);">
      {{ $t('decks.create.searching') }}
    </p>

    <ul
      v-if="isOpen && suggestions.length"
      :id="listboxId"
      role="listbox"
      :aria-label="label"
      class="absolute left-0 right-0 top-[calc(100%+6px)] z-40 max-h-[260px] overflow-y-auto rounded-[var(--radius-md)] border p-1.5 shadow-[0_16px_36px_rgba(0,0,0,0.3)]"
      style="background: var(--menu-bg); border-color: var(--card-border);"
    >
      <li
        v-for="(suggestion, index) in suggestions"
        :id="optionId(index)"
        :key="suggestion.name"
        role="option"
        :aria-selected="index === activeIndex"
        class="flex cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] px-2.5 py-2 transition-colors"
        :style="index === activeIndex ? 'background: rgba(139,92,246,0.18);' : ''"
        @mouseenter="activeIndex = index"
        @mousedown.prevent="choose(suggestion)"
      >
        <img
          v-if="suggestion.image_url"
          :src="suggestion.image_url"
          alt=""
          class="h-8 w-12 shrink-0 rounded-[var(--radius-sm)] object-cover"
        >
        <span
          v-else
          aria-hidden="true"
          class="h-8 w-12 shrink-0 rounded-[var(--radius-sm)]"
          style="background: linear-gradient(160deg, rgba(139,92,246,0.35), rgba(168,85,247,0.15));"
        />
        <span class="min-w-0">
          <span class="block truncate text-[13px]" style="color: var(--text);">{{ suggestion.name }}</span>
          <span class="block truncate text-[11px]" style="color: var(--text-dim);">{{ suggestion.type_line }}</span>
        </span>
      </li>
    </ul>

    <p
      v-else-if="isOpen && hasSearched && !isLoading"
      class="mt-1.5 px-4 text-[11px]"
      style="color: var(--text-dim);"
    >
      {{ $t('decks.create.noSuggestions') }}
    </p>
  </div>
</template>
