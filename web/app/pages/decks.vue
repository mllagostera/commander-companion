<script setup lang="ts">
import type { CommanderSuggestion } from '#shared/types/scryfall'
import type { Deck, DeckResyncJob, DeckStats, DeckTraitFilter, ManaColor, PaginatedResponse } from '~/types/api'

const { t } = useI18n()
const {
  listDecksPage,
  createDeck,
  updateDeck,
  importFromMoxfield,
  syncFromMoxfield,
  resyncAllDecks,
  getResyncAllStatus,
} = useDecks()
const { allDeckStats } = useStatistics()
const { showToast } = useToast()

// --------------------------------------------------------- paginated list
// The API returns decks a page at a time (default 20). The grid loads pages
// lazily as the user scrolls (see scrollSentinel/IntersectionObserver
// below); a search, though, has to be able to match decks that haven't
// scrolled into view yet, so typing into the search box eagerly fetches
// every remaining page instead of only filtering what's already loaded.
//
// The bracket/color filter, unlike the search, runs server-side: changing it
// refetches from the first page (useAsyncData's `watch`), and every later page
// carries the same filter.
const traitFilter = ref<DeckTraitFilter>(emptyDeckTraitFilter())
const { data: firstPage, refresh: refreshFirstPage, error: listError } = await useAsyncData<PaginatedResponse<Deck>>(
  'decks-first-page',
  () => listDecksPage(undefined, traitFilter.value),
  { default: () => ({ items: [], next_cursor: null }), watch: [traitFilter] },
)

const decks = ref<Deck[]>([])
const nextCursor = ref<string | null>(null)
const isLoadingMore = ref(false)

// Which listing `decks`/`nextCursor` belong to. Bumped whenever a listing ends
// (new filter) or starts (a first page lands), so a page fetched for an
// earlier listing is dropped instead of appended, whichever order the
// responses come back in.
let listing = 0

function syncFromFirstPage(page: PaginatedResponse<Deck> | null | undefined) {
  listing++
  decks.value = page?.items ?? []
  nextCursor.value = page?.next_cursor ?? null
}

watch(firstPage, syncFromFirstPage, { immediate: true })

// The old listing's cursor means nothing under the new filter: drop it right
// away, so neither scrolling nor a search asks for another page until the new
// first page brings its own. 'sync' so this runs before anything else can.
watch(traitFilter, () => {
  listing++
  nextCursor.value = null
}, { flush: 'sync' })

async function loadMore() {
  if (isLoadingMore.value || !nextCursor.value) return
  isLoadingMore.value = true
  const listingAtStart = listing
  try {
    const page = await listDecksPage(nextCursor.value, traitFilter.value)
    if (listing !== listingAtStart) return
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

// Guards against concurrent loadAllRemaining() calls. Without this, every
// caller starts its own `while (nextCursor.value)` loop over the same
// nextCursor/isLoadingMore state; only one of them ever does real work
// (the isLoadingMore check inside loadMore short-circuits the rest), but
// the others don't wait for it either -- they just busy-spin re-checking
// isLoadingMore as fast as microtasks allow until it clears. With enough
// decks (many pages) and more than one trigger in flight (e.g. a few
// keystrokes before the debounce below fires, or a search overlapping a
// post-import refresh), that spin pegs the tab and the page looks frozen.
// Sharing one in-flight promise means extra callers just await it instead.
let loadAllRemainingPromise: Promise<void> | null = null
function loadAllRemainingOnce(): Promise<void> {
  if (!loadAllRemainingPromise) {
    loadAllRemainingPromise = loadAllRemaining().finally(() => {
      loadAllRemainingPromise = null
    })
  }
  return loadAllRemainingPromise
}

// Doesn't rely on the `watch` above to have already synced `decks`/`nextCursor`
// by the time this continues (its flush timing isn't guaranteed relative to
// this function resuming after the `await`) -- syncs explicitly instead.
async function refresh() {
  await Promise.all([refreshFirstPage(), refreshStats()])
  syncFromFirstPage(firstPage.value)
  if (deckSearch.value.trim()) await loadAllRemainingOnce()
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

// ------------------------------------------------------ create by hand
// Two ways to add a deck, one entry point: the header's primary button opens
// this form, and the Moxfield import hangs off it as a secondary action
// (openImportModal below) rather than as a third button competing for the
// same corner of the page.
const isCreateModalOpen = ref(false)
const createModalRef = ref<HTMLElement | null>(null)
const newDeckName = ref('')
const newDeckCommander = ref('')
const createError = ref('')
const isCreating = ref(false)
const pickedCommander = ref<CommanderSuggestion | null>(null)
const newDeckBracket = ref<number | null>(null)
const newDeckColors = ref<ManaColor[] | null>(null)

/**
 * The picked commander's art, but only while the field still holds that
 * commander's name — edit it by hand afterwards and the deck goes back to
 * DeckArt's placeholder rather than keeping one commander's art under
 * another's name.
 *
 * Derived rather than cleared by a watcher on purpose: picking a suggestion
 * emits `update:modelValue` and `select` in the same tick, so a default
 * (flush: 'pre') watcher would run *after* the art was set and wipe it.
 */
const newDeckImageUrl = computed(() =>
  pickedCommander.value?.name === newDeckCommander.value.trim()
    ? pickedCommander.value?.image_url ?? null
    : null,
)

function openCreateModal() {
  newDeckName.value = ''
  newDeckCommander.value = ''
  pickedCommander.value = null
  newDeckBracket.value = null
  newDeckColors.value = null
  createError.value = ''
  isCreateModalOpen.value = true
}

function closeCreateModal() {
  isCreateModalOpen.value = false
}

useModalA11y(isCreateModalOpen, createModalRef, closeCreateModal)

const canSubmitCreate = computed(() => !!newDeckName.value.trim() && !!newDeckCommander.value.trim())

async function handleCreate() {
  if (!canSubmitCreate.value) return
  createError.value = ''
  isCreating.value = true
  try {
    await createDeck({
      name: newDeckName.value,
      commander: newDeckCommander.value,
      imageUrl: newDeckImageUrl.value,
      bracket: newDeckBracket.value,
      colorIdentity: newDeckColors.value,
    })
    closeCreateModal()
    await refresh()
    showToast(t('toast.deckCreated'))
  } catch (err) {
    createError.value = createDeckError(err)
  } finally {
    isCreating.value = false
  }
}

const isImportModalOpen = ref(false)
const moxfieldInput = ref('')
const importError = ref('')
const importedDeck = ref<Deck | null>(null)
const isImporting = ref(false)
const importModalRef = ref<HTMLElement | null>(null)

function openImportModal() {
  moxfieldInput.value = ''
  importError.value = ''
  importedDeck.value = null
  isCreateModalOpen.value = false
  isImportModalOpen.value = true
}

function closeImportModal() {
  isImportModalOpen.value = false
}

useModalA11y(isImportModalOpen, importModalRef, closeImportModal)

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

// ------------------------------------------- edit bracket / color identity
// Whatever is saved here is flagged as set by hand, so "Actualizar" keeps it.
// On a Moxfield deck, "use Moxfield's" drops that and takes Moxfield's value
// right away (the backend fetches it in the same request).
const editingDeck = ref<Deck | null>(null)
const editModalRef = ref<HTMLElement | null>(null)
const editBracket = ref<number | null>(null)
const editColors = ref<ManaColor[] | null>(null)
const editError = ref('')
const isSavingEdit = ref(false)
const isEditModalOpen = computed(() => editingDeck.value !== null)

function openEditModal(deck: Deck) {
  editingDeck.value = deck
  editBracket.value = deck.bracket
  editColors.value = deck.color_identity ? [...deck.color_identity] : null
  editError.value = ''
}

function closeEditModal() {
  editingDeck.value = null
}

useModalA11y(isEditModalOpen, editModalRef, closeEditModal)

function replaceDeck(updated: Deck) {
  const idx = decks.value.findIndex((d) => d.id === updated.id)
  if (idx !== -1) decks.value[idx] = updated
}

function sameColors(a: ManaColor[] | null, b: ManaColor[] | null): boolean {
  return a === null || b === null ? a === b : a.join('') === b.join('')
}

async function saveEdit() {
  const deck = editingDeck.value
  if (!deck) return
  const body: Parameters<typeof updateDeck>[1] = {}
  if (editBracket.value !== deck.bracket) body.bracket = editBracket.value
  if (!sameColors(editColors.value, deck.color_identity)) body.color_identity = editColors.value
  if (!Object.keys(body).length) {
    closeEditModal()
    return
  }
  await submitEdit(deck, body, true)
}

async function resetEdit(field: 'bracket' | 'color_identity') {
  const deck = editingDeck.value
  if (!deck) return
  await submitEdit(deck, field === 'bracket' ? { reset_bracket: true } : { reset_color_identity: true }, false)
}

async function submitEdit(deck: Deck, body: Parameters<typeof updateDeck>[1], closeAfter: boolean) {
  editError.value = ''
  isSavingEdit.value = true
  try {
    const updated = await updateDeck(deck.id, body)
    replaceDeck(updated)
    if (closeAfter) {
      closeEditModal()
      showToast(t('toast.deckUpdated'))
    } else {
      openEditModal(updated)
    }
  } catch (err) {
    editError.value = updateDeckError(err)
  } finally {
    isSavingEdit.value = false
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
type SortKey = 'played' | 'won' | 'winRate' | 'name'
const deckSearch = ref('')
const deckSort = ref<SortKey>('played')
const deckSortOptions = computed(() => [
  { value: 'played', label: t('decks.sort.played') },
  { value: 'won', label: t('decks.sort.won') },
  { value: 'winRate', label: t('decks.sort.winRate') },
  { value: 'name', label: t('decks.sort.name') },
])

// A search has to match against every deck, not just the ones already
// scrolled into view: once the user pauses typing, fetch whatever pages
// are still missing instead of relying on scroll to bring them in
// eventually. Debounced the same way as the playgroup member search
// (playgroups/[id].vue) so typing doesn't fire one loadAllRemaining per
// keystroke.
let searchDebounce: ReturnType<typeof setTimeout> | undefined
watch(deckSearch, (q) => {
  clearTimeout(searchDebounce)
  if (!q.trim()) return
  searchDebounce = setTimeout(() => loadAllRemainingOnce(), 300)
})

onUnmounted(() => clearTimeout(searchDebounce))

// A new trait filter starts the listing over from page 1; if a search is
// active it still has to see every page of the new listing.
watch(firstPage, () => {
  if (deckSearch.value.trim()) loadAllRemainingOnce()
})

function statsFor(deck: Deck): DeckStats | null {
  return statsByDeckId.value.get(deck.id) ?? null
}

const isTraitFilterActive = computed(() => isDeckTraitFilterActive(traitFilter.value))

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
    if (deckSort.value === 'winRate') {
      const ra = sa?.games_played ? sa.games_won / sa.games_played : 0
      const rb = sb?.games_played ? sb.games_won / sb.games_played : 0
      const diff = rb - ra
      return diff !== 0 ? diff : (sb?.games_played ?? 0) - (sa?.games_played ?? 0)
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
          @click="openCreateModal"
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
        :aria-label="$t('decks.searchPlaceholder')"
        class="min-w-[200px] flex-1 rounded-full border px-4 py-2.5 text-[13px] outline-none"
        style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
      >
      <SortSelect
        :model-value="deckSort"
        :options="deckSortOptions"
        :select-label="$t('decks.sort.ariaLabel')"
        @update:model-value="(v) => (deckSort = v as SortKey)"
      />
    </section>

    <DeckTraitFilterBar v-model="traitFilter" />

    <p v-if="listError" class="text-sm" style="color: var(--lose);">{{ $t('decks.loadError') }}</p>
    <!-- Two different "nothing to show" cases: an account with no decks at all
         (offer the import) versus a search or filter that matched none of them
         (offering the import there would be answering a question nobody asked). -->
    <EmptyState
      v-else-if="!decks?.length && !isTraitFilterActive"
      :title="$t('decks.emptyTitle')"
      :body="$t('decks.emptyBody')"
      :cta-label="$t('decks.addDeck')"
      @cta="openCreateModal"
    />
    <EmptyState
      v-else-if="!filteredDecks.length"
      :title="$t('decks.noMatchesTitle')"
      :body="isTraitFilterActive ? $t('decks.noFilterMatchesBody') : $t('decks.noMatchesBody')"
    />

    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div v-for="deck in filteredDecks" :key="deck.id" class="relative">
        <DeckArt :deck="deck" aspect-ratio="21/9" rounded="rounded-[var(--radius-lg)]" image-position="right" />
        <div
          class="pointer-events-none absolute inset-0 rounded-[var(--radius-lg)]"
          style="background: linear-gradient(90deg, rgba(10,7,20,0.94) 0%, rgba(10,7,20,0.82) 38%, rgba(10,7,20,0.25) 68%, rgba(10,7,20,0) 92%);"
        />
        <div class="absolute inset-y-0 left-0 flex w-[68%] flex-col justify-between p-4 sm:w-[58%]">
          <div class="pointer-events-none">
            <p class="font-semibold text-white">{{ deck.name }}</p>
            <p class="mt-1 text-xs text-white/70">{{ deck.commander }}</p>
            <DeckTraits :bracket="deck.bracket" :colors="deck.color_identity" on-art class="mt-1.5" />
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
            <button
              type="button"
              class="rounded-full border border-white/25 px-2.5 py-1 text-xs text-white/90 hover:bg-white/10"
              :aria-label="$t('deckTraits.edit.actionFor', { name: deck.name })"
              @click="openEditModal(deck)"
            >
              {{ $t('deckTraits.edit.action') }}
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
      v-if="isCreateModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeCreateModal"
    >
      <div
        ref="createModalRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="decks-create-title"
        class="w-full max-w-sm rounded-[var(--radius-xl)] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <div class="flex items-center justify-between">
          <h2 id="decks-create-title" class="text-[15px] font-medium">{{ $t('decks.create.title') }}</h2>
          <button
            type="button"
            :aria-label="$t('common.close')"
            class="-m-2 p-2 text-sm"
            style="color: var(--text-dim);"
            @click="closeCreateModal"
          >
            <span aria-hidden="true">✕</span>
          </button>
        </div>

        <form class="mt-4 flex flex-col gap-3" @submit.prevent="handleCreate">
          <div class="flex flex-col gap-1.5">
            <label for="new-deck-name" class="px-1 text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
              {{ $t('decks.create.nameLabel') }}
            </label>
            <input
              id="new-deck-name"
              v-model="newDeckName"
              type="text"
              required
              autofocus
              :placeholder="$t('decks.create.namePlaceholder')"
              class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
            >
          </div>

          <div class="flex flex-col gap-1.5">
            <label for="new-deck-commander" class="px-1 text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
              {{ $t('decks.create.commanderLabel') }}
            </label>
            <CommanderSearch
              v-model="newDeckCommander"
              input-id="new-deck-commander"
              :label="$t('decks.create.commanderLabel')"
              @select="(s) => (pickedCommander = s)"
            />
          </div>

          <!-- Confirms the pick did more than fill in text: this art is what
               the deck card will show in the grid. -->
          <div
            v-if="newDeckImageUrl"
            class="flex items-center gap-3 rounded-[var(--radius-md)] border p-2.5"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <img
              :src="newDeckImageUrl"
              alt=""
              class="h-10 w-16 shrink-0 rounded-[var(--radius-sm)] object-cover"
            >
            <p class="text-[11px]" style="color: var(--text-muted);">{{ $t('decks.create.artAttached') }}</p>
          </div>

          <DeckTraitsEditor v-model:bracket="newDeckBracket" v-model:colors="newDeckColors" id-prefix="new-deck" />

          <button
            type="submit"
            :disabled="isCreating || !canSubmitCreate"
            class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.02] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ isCreating ? $t('decks.create.submitting') : $t('decks.create.submit') }}
          </button>
        </form>

        <p v-if="createError" class="mt-3 text-sm" style="color: var(--lose);">{{ createError }}</p>

        <p class="mt-4 border-t pt-4 text-[13px]" style="border-color: var(--card-border); color: var(--text-muted);">
          {{ $t('decks.create.moxfieldPrompt') }}
          <!-- Padded to clear SC 2.5.8's 24px floor without moving the text:
               it sits inline in a sentence, so it can't just grow. -->
          <button
            type="button"
            class="-my-1 inline-block py-1 underline"
            style="color: var(--accent-link);"
            @click="openImportModal"
          >
            {{ $t('decks.create.moxfieldAction') }}
          </button>
        </p>
      </div>
    </div>

    <div
      v-if="editingDeck"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeEditModal"
    >
      <div
        ref="editModalRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="decks-edit-title"
        class="w-full max-w-sm rounded-[var(--radius-xl)] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <div class="flex items-center justify-between">
          <div class="min-w-0">
            <h2 id="decks-edit-title" class="text-[15px] font-medium">{{ $t('deckTraits.edit.title') }}</h2>
            <p class="mt-0.5 truncate text-[12px]" style="color: var(--text-muted);">{{ editingDeck.name }}</p>
          </div>
          <button
            type="button"
            :aria-label="$t('common.close')"
            class="-m-2 p-2 text-sm"
            style="color: var(--text-dim);"
            @click="closeEditModal"
          >
            <span aria-hidden="true">✕</span>
          </button>
        </div>

        <form class="mt-4 flex flex-col gap-4" @submit.prevent="saveEdit">
          <DeckTraitsEditor v-model:bracket="editBracket" v-model:colors="editColors" id-prefix="edit-deck" />

          <!-- Only a Moxfield deck has a value to go back to, and only one set
               by hand needs going back. -->
          <div
            v-if="editingDeck.moxfield_id && (editingDeck.bracket_overridden || editingDeck.color_identity_overridden)"
            class="flex flex-col items-start gap-1 rounded-[var(--radius-md)] border p-3 text-[12px]"
            style="border-color: var(--card-border); background: var(--card-bg); color: var(--text-muted);"
          >
            <p>{{ $t('deckTraits.edit.overriddenHint') }}</p>
            <button
              v-if="editingDeck.bracket_overridden"
              type="button"
              :disabled="isSavingEdit"
              class="py-1 underline disabled:opacity-50"
              style="color: var(--accent-link);"
              @click="resetEdit('bracket')"
            >
              {{ $t('deckTraits.edit.resetBracket') }}
            </button>
            <button
              v-if="editingDeck.color_identity_overridden"
              type="button"
              :disabled="isSavingEdit"
              class="py-1 underline disabled:opacity-50"
              style="color: var(--accent-link);"
              @click="resetEdit('color_identity')"
            >
              {{ $t('deckTraits.edit.resetColors') }}
            </button>
          </div>

          <button
            type="submit"
            :disabled="isSavingEdit"
            class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] transition-transform hover:scale-[1.02] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ isSavingEdit ? $t('deckTraits.edit.saving') : $t('deckTraits.edit.save') }}
          </button>
        </form>

        <p v-if="editError" class="mt-3 text-sm" style="color: var(--lose);">{{ editError }}</p>
      </div>
    </div>

    <div
      v-if="isImportModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeImportModal"
    >
      <div
        ref="importModalRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="decks-import-title"
        class="w-full max-w-sm rounded-[var(--radius-xl)] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <div class="flex items-center justify-between">
          <h2 id="decks-import-title" class="text-[15px] font-medium">{{ $t('decks.import.title') }}</h2>
          <button
            type="button"
            :aria-label="$t('common.close')"
            class="-m-2 p-2 text-sm"
            style="color: var(--text-dim);"
            @click="closeImportModal"
          >
            <span aria-hidden="true">✕</span>
          </button>
        </div>

        <form class="mt-4 flex flex-col gap-3" @submit.prevent="handleImport">
          <input
            v-model="moxfieldInput"
            type="text"
            required
            autofocus
            :placeholder="$t('decks.import.placeholder')"
            :aria-label="$t('decks.import.placeholder')"
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
          class="mt-4 flex gap-4 rounded-[var(--radius-md)] border p-4"
          style="border-color: rgba(52,211,153,0.35); background: var(--win-bg);"
        >
          <img
            v-if="importedDeck.image_url"
            :src="importedDeck.image_url"
            :alt="importedDeck.commander"
            class="h-16 w-16 shrink-0 rounded-[var(--radius-sm)] object-cover"
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
