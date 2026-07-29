<script setup lang="ts">
import type { Deck, DeckStats } from '~/types/api'

const { t } = useI18n()
const { listDecks, importFromMoxfield, syncFromMoxfield } = useDecks()
const { deckStats } = useStatistics()
const { showToast } = useToast()

const { data: decks, refresh, error: listError } = await useAsyncData<Deck[]>(
  'decks',
  () => listDecks(),
  { default: () => [] },
)

// Stats por deck, solo para ordenar (jugados/victorias/win rate) — el propio
// Deck no las trae. Best-effort: un deck sin stats no rompe el resto de la lista.
const statsByDeckId = ref<Record<string, DeckStats | null>>({})

watch(
  decks,
  async (list) => {
    if (!list) return
    const entries = await Promise.all(
      list.map(async (d) => [d.id, await deckStats(d.id).catch(() => null)] as const),
    )
    statsByDeckId.value = Object.fromEntries(entries)
  },
  { immediate: true },
)

const isImportModalOpen = ref(false)
const moxfieldInput = ref('')
const importError = ref('')
const importedDeck = ref<Deck | null>(null)
const isImporting = ref(false)

function openImportModal() {
  moxfieldInput.value = ''
  importError.value = ''
  importedDeck.value = null
  isImportModalOpen.value = true
}

function closeImportModal() {
  isImportModalOpen.value = false
}

async function handleImport() {
  importError.value = ''
  importedDeck.value = null
  isImporting.value = true
  try {
    importedDeck.value = await importFromMoxfield(moxfieldInput.value)
    moxfieldInput.value = ''
    await refresh()
    showToast(t('toast.deckImported'))
  } catch (err) {
    importError.value = moxfieldImportError(err)
  } finally {
    isImporting.value = false
  }
}

// Estado de sincronización por deck (id → mensaje/loading/error), independiente
// entre filas: sincronizar un deck no debe bloquear ni pisar el estado de otro.
const syncState = reactive<Record<string, { loading: boolean, message: string, isError: boolean }>>({})

async function handleSync(deck: Deck) {
  if (!deck.moxfield_id) return
  syncState[deck.id] = { loading: true, message: '', isError: false }
  try {
    const res = await syncFromMoxfield(deck.moxfield_id)
    const idx = decks.value?.findIndex((d) => d.id === deck.id) ?? -1
    if (idx !== -1 && decks.value) decks.value[idx] = res.deck
    syncState[deck.id] = {
      loading: false,
      isError: false,
      message: res.status === 'updated' ? t('decks.sync.updated') : t('decks.sync.upToDate'),
    }
  } catch (err) {
    syncState[deck.id] = {
      loading: false,
      isError: true,
      message: apiErrorMessage(err, t('decks.errors.syncFailed')),
    }
  }
}

// ------------------------------------------------------- búsqueda y orden
type SortKey = 'played' | 'won' | 'winrate' | 'name'
const deckSearch = ref('')
const deckSort = ref<SortKey>('played')

function statsFor(deck: Deck): DeckStats | null {
  return statsByDeckId.value[deck.id] ?? null
}

const filteredDecks = computed(() => {
  const q = deckSearch.value.trim().toLowerCase()
  const list = (decks.value ?? []).filter(
    (d) => !q || d.name.toLowerCase().includes(q) || d.commander.toLowerCase().includes(q),
  )
  return [...list].sort((a, b) => {
    if (deckSort.value === 'name') return a.name.localeCompare(b.name)
    const sa = statsFor(a)
    const sb = statsFor(b)
    if (deckSort.value === 'won') return (sb?.games_won ?? 0) - (sa?.games_won ?? 0)
    if (deckSort.value === 'winrate') {
      const ra = sa?.games_played ? sa.games_won / sa.games_played : 0
      const rb = sb?.games_played ? sb.games_won / sb.games_played : 0
      return rb - ra
    }
    return (sb?.games_played ?? 0) - (sa?.games_played ?? 0)
  })
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <section class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('decks.title') }}</h1>
        <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('decks.subtitle') }}</p>
      </div>
      <button
        type="button"
        class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
        style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
        @click="openImportModal"
      >
        {{ $t('decks.addDeck') }}
      </button>
    </section>

    <section class="flex flex-wrap items-center gap-2.5">
      <input
        v-model="deckSearch"
        type="text"
        :placeholder="$t('decks.searchPlaceholder')"
        class="min-w-[200px] flex-1 rounded-full border px-4 py-2.5 text-[13px] outline-none"
        style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
      >
      <select
        v-model="deckSort"
        class="rounded-full border px-4 py-2.5 text-[13px]"
        style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
      >
        <option value="played">{{ $t('decks.sort.played') }}</option>
        <option value="won">{{ $t('decks.sort.won') }}</option>
        <option value="winrate">{{ $t('decks.sort.winrate') }}</option>
        <option value="name">{{ $t('decks.sort.name') }}</option>
      </select>
    </section>

    <p v-if="listError" class="text-sm" style="color: var(--lose);">{{ $t('decks.loadError') }}</p>
    <p v-else-if="!filteredDecks.length" class="text-sm" style="color: var(--text-muted);">
      {{ $t('decks.empty') }}
    </p>

    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div v-for="deck in filteredDecks" :key="deck.id" class="relative">
        <DeckArt :deck="deck" aspect-ratio="21/9" rounded="rounded-[22px]" />
        <div class="pointer-events-none absolute inset-0 rounded-[22px]" style="background: linear-gradient(180deg, rgba(10,7,20,0.08) 25%, rgba(10,7,20,0.92) 100%);" />
        <div class="absolute inset-x-0 bottom-0 p-4">
          <div class="pointer-events-none">
            <p class="font-semibold text-white">{{ deck.name }}</p>
            <p class="mt-1 text-xs text-white/70">{{ deck.commander }}</p>
            <p v-if="statsFor(deck)" class="mt-1 text-[11px] text-white/60">
              {{ $t('decks.stats', { played: statsFor(deck)!.games_played, won: statsFor(deck)!.games_won }) }}
            </p>
          </div>
          <div class="pointer-events-auto mt-2 flex flex-wrap items-center justify-between gap-2">
            <span v-if="deck.moxfield_id" class="flex items-center gap-2">
              <button
                type="button"
                :disabled="syncState[deck.id]?.loading"
                class="rounded-full border border-white/25 px-2.5 py-1 text-xs text-white/90 hover:bg-white/10 disabled:opacity-50"
                @click="handleSync(deck)"
              >
                {{ syncState[deck.id]?.loading ? $t('decks.sync.syncing') : $t('decks.sync.action') }}
              </button>
              <span
                v-if="syncState[deck.id]?.message"
                class="text-xs"
                :style="{ color: syncState[deck.id]?.isError ? '#fca5a5' : '#86efac' }"
              >
                {{ syncState[deck.id]?.message }}
              </span>
            </span>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="isImportModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeImportModal"
    >
      <div class="w-full max-w-sm rounded-[24px] border p-6" style="border-color: var(--card-border); background: var(--page-solid);">
        <div class="flex items-center justify-between">
          <h2 class="text-[15px] font-medium">{{ $t('decks.import.title') }}</h2>
          <button
            type="button"
            class="p-0 text-sm"
            style="color: var(--text-dim);"
            @click="closeImportModal"
          >
            ✕
          </button>
        </div>

        <form class="mt-4 flex flex-col gap-3" @submit.prevent="handleImport">
          <input
            v-model="moxfieldInput"
            type="text"
            required
            autofocus
            :placeholder="$t('decks.import.placeholder')"
            class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
          <button
            type="submit"
            :disabled="isImporting"
            class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.02] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ isImporting ? $t('decks.import.submitting') : $t('decks.import.submit') }}
          </button>
        </form>

        <p v-if="importError" class="mt-3 text-sm" style="color: var(--lose);">{{ importError }}</p>

        <div
          v-if="importedDeck"
          class="mt-4 flex gap-4 rounded-[18px] border p-4"
          style="border-color: rgba(52,211,153,0.35); background: var(--win-bg);"
        >
          <img
            v-if="importedDeck.image_url"
            :src="importedDeck.image_url"
            :alt="importedDeck.commander"
            class="h-16 w-16 shrink-0 rounded-[14px] object-cover"
          >
          <div>
            <p class="text-sm" style="color: var(--win);">{{ $t('toast.deckImported') }}</p>
            <p class="mt-1 font-medium">{{ importedDeck.name }}</p>
            <p class="text-sm" style="color: var(--text-muted);">{{ $t('decks.import.commanderLabel', { commander: importedDeck.commander }) }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
