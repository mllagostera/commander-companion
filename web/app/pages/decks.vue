<script setup lang="ts">
import type { Deck, DeckResyncJob, DeckStats, PaginatedResponse } from '~/types/api'

const { t } = useI18n()
const { listDecksPage, importFromMoxfield, syncFromMoxfield, resyncAllDecks, getResyncAllStatus } = useDecks()
const { allDeckStats } = useStatistics()
const { showToast } = useToast()

// --------------------------------------------------------- paginated list
// The API returns decks a page at a time (default 20). The grid loads pages
// lazily as the user scrolls (see scrollSentinel/IntersectionObserver
// below); a search, though, has to be able to match decks that haven't
// scrolled into view yet, so typing into the search box eagerly fetches
// every remaining page instead of only filtering what's already loaded.
const { data: firstPage, refresh: refreshFirstPage, error: listError } = await useAsyncData<PaginatedResponse<Deck>>(
  'decks-first-page',
  () => listDecksPage(),
  { default: () => ({ items: [], next_cursor: null }) },
)

const decks = ref<Deck[]>([])
const nextCursor = ref<string | null>(null)
const isLoadingMore = ref(false)

function syncFromFirstPage(page: PaginatedResponse<Deck> | null | undefined) {
  decks.value = page?.items ?? []
  nextCursor.value = page?.next_cursor ?? null
}

watch(firstPage, syncFromFirstPage, { immediate: true })

async function loadMore() {
  if (isLoadingMore.value || !nextCursor.value) return
  isLoadingMore.value = true
  try {
    const page = await listDecksPage(nextCursor.value)
    decks.value = [...decks.value, ...page.items]
    nextCursor.value = page.next_cursor
  } finally {
    isLoadingMore.value = false
  }
}

async function loadAllRemaining() {
  while (nextCursor.value) {
    await loadMore()
  }
}

// Doesn't rely on the `watch` above to have already synced `decks`/`nextCursor`
// by the time this continues (its flush timing isn't guaranteed relative to
// this function resuming after the `await`) -- syncs explicitly instead.
async function refresh() {
  await Promise.all([refreshFirstPage(), refreshStats()])
  syncFromFirstPage(firstPage.value)
  if (deckSearch.value.trim()) await loadAllRemaining()
}

const scrollSentinel = ref<HTMLElement | null>(null)
let scrollObserver: IntersectionObserver | null = null

onMounted(() => {
  scrollObserver = new IntersectionObserver((entries) => {
    if (entries[0]?.isIntersecting) loadMore()
  })
  if (scrollSentinel.value) scrollObserver.observe(scrollSentinel.value)
})

watch(scrollSentinel, (el, previousEl) => {
  if (previousEl) scrollObserver?.unobserve(previousEl)
  if (el) scrollObserver?.observe(el)
})

onUnmounted(() => scrollObserver?.disconnect())

// Stats per deck, only used for sorting (played/wins/win rate) — the Deck
// itself doesn't carry them. Fetched once for every deck up front (not
// incrementally as pages load): a single request, independent of how many
// pages the grid ends up loading via scroll/search.
const { data: statsList, refresh: refreshStats } = await useAsyncData('decks-stats', () => allDeckStats(), { default: () => [] })
const statsByDeckId = computed(() => new Map((statsList.value ?? []).map((s) => [s.deck_id, s])))

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

// Sync state per deck (id → message/loading/error), independent
// between rows: syncing one deck must not block or overwrite another's state.
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

// --------------------------------------------------- resync all decks
const resyncJob = ref<DeckResyncJob | null>(null)
const resyncError = ref('')
const isStartingResync = ref(false)
let resyncPollHandle: ReturnType<typeof setTimeout> | null = null

function stopResyncPolling() {
  if (resyncPollHandle) clearTimeout(resyncPollHandle)
  resyncPollHandle = null
}

function pollResyncStatus(jobId: string) {
  stopResyncPolling()
  resyncPollHandle = setTimeout(async () => {
    try {
      resyncJob.value = await getResyncAllStatus(jobId)
    } catch {
      // Best-effort: a one-off network error shouldn't stop polling.
    }
    if (resyncJob.value?.status === 'in_progress') {
      pollResyncStatus(jobId)
    } else {
      await refresh()
    }
  }, 2000)
}

async function handleResyncAll() {
  resyncError.value = ''
  isStartingResync.value = true
  try {
    resyncJob.value = await resyncAllDecks()
    pollResyncStatus(resyncJob.value.id)
  } catch (err) {
    resyncError.value = resyncAllDecksError(err)
  } finally {
    isStartingResync.value = false
  }
}

onUnmounted(stopResyncPolling)

// ------------------------------------------------------- search and sort
type SortKey = 'played' | 'won' | 'winrate' | 'name'
const deckSearch = ref('')
const deckSort = ref<SortKey>('played')

// A search has to match against every deck, not just the ones already
// scrolled into view: as soon as there's a query, fetch whatever pages are
// still missing instead of relying on scroll to bring them in eventually.
watch(deckSearch, (q) => {
  if (q.trim()) loadAllRemaining()
})

function statsFor(deck: Deck): DeckStats | null {
  return statsByDeckId.value.get(deck.id) ?? null
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
      <div class="flex flex-wrap items-center gap-2.5">
        <button
          type="button"
          :disabled="isStartingResync || resyncJob?.status === 'in_progress'"
          class="rounded-full border px-4 py-2.5 text-[13px] disabled:opacity-50"
          style="border-color: var(--input-border); color: var(--text);"
          @click="handleResyncAll"
        >
          {{ isStartingResync ? $t('decks.resyncAll.starting') : $t('decks.resyncAll.action') }}
        </button>
        <button
          type="button"
          class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          @click="openImportModal"
        >
          {{ $t('decks.addDeck') }}
        </button>
      </div>
    </section>

    <section v-if="resyncError || resyncJob" class="text-sm" style="color: var(--text-muted);">
      <p v-if="resyncError" style="color: var(--lose);">{{ resyncError }}</p>
      <p v-else-if="resyncJob?.status === 'in_progress'">
        {{ $t('decks.resyncAll.inProgress', { done: resyncJob.updated_count + resyncJob.failed_count, total: resyncJob.total_decks }) }}
      </p>
      <p v-else-if="resyncJob?.status === 'completed'" style="color: var(--win);">
        {{ $t('decks.resyncAll.completed', { updated: resyncJob.updated_count, failed: resyncJob.failed_count }) }}
      </p>
      <p v-else-if="resyncJob?.status === 'failed'" style="color: var(--lose);">
        {{ $t('decks.resyncAll.failed', { message: resyncJob.error_message }) }}
      </p>
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
        <DeckArt :deck="deck" aspect-ratio="21/9" rounded="rounded-[22px]" image-position="right" />
        <div
          class="pointer-events-none absolute inset-0 rounded-[22px]"
          style="background: linear-gradient(90deg, rgba(10,7,20,0.94) 0%, rgba(10,7,20,0.82) 38%, rgba(10,7,20,0.25) 68%, rgba(10,7,20,0) 92%);"
        />
        <div class="absolute inset-y-0 left-0 flex w-[68%] flex-col justify-between p-4 sm:w-[58%]">
          <div class="pointer-events-none">
            <p class="font-semibold text-white">{{ deck.name }}</p>
            <p class="mt-1 text-xs text-white/70">{{ deck.commander }}</p>
            <p v-if="statsFor(deck)" class="mt-1 text-[11px] text-white/60">
              {{ $t('decks.stats', { played: statsFor(deck)!.games_played, won: statsFor(deck)!.games_won }) }}
            </p>
          </div>
          <div class="pointer-events-auto flex flex-wrap items-center gap-2">
            <a
              v-if="deck.moxfield_id"
              :href="`https://moxfield.com/decks/${deck.moxfield_id}`"
              target="_blank"
              rel="noopener noreferrer"
              class="rounded-full border border-white/25 px-2.5 py-1 text-xs text-white/90 hover:bg-white/10"
            >
              {{ $t('decks.viewOnMoxfield') }}
            </a>
            <button
              v-if="deck.moxfield_id"
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
          </div>
        </div>
      </div>
    </div>

    <div v-if="nextCursor" ref="scrollSentinel" class="h-4 w-full" />
    <p v-if="isLoadingMore" class="text-center text-xs" style="color: var(--text-dim);">
      {{ $t('decks.loadingMore') }}
    </p>

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
