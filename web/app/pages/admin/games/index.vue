<script setup lang="ts">
import type { AdminUnfinishedGame } from '~/types/api'

definePageMeta({ middleware: 'admin' })

type StatusFilter = 'all' | 'pending' | 'active'

const { t, d, locale } = useI18n()
const { listUnfinishedGames, deleteUnfinishedGame } = useAdmin()

const statusFilters: StatusFilter[] = ['all', 'pending', 'active']
const statusFilter = ref<StatusFilter>('all')
const games = ref<AdminUnfinishedGame[]>([])
const cursor = ref<string | null>(null)
const isLoading = ref(false)
const isLoadingMore = ref(false)
const loadError = ref(false)

function statusQuery() {
  return statusFilter.value === 'all' ? undefined : statusFilter.value
}

async function loadGames() {
  isLoading.value = true
  loadError.value = false
  try {
    const page = await listUnfinishedGames(undefined, statusQuery())
    games.value = page.items
    cursor.value = page.next_cursor
  } catch {
    loadError.value = true
  } finally {
    isLoading.value = false
  }
}

async function loadMore() {
  if (!cursor.value) return
  isLoadingMore.value = true
  try {
    const page = await listUnfinishedGames(cursor.value, statusQuery())
    games.value = [...games.value, ...page.items]
    cursor.value = page.next_cursor
  } finally {
    isLoadingMore.value = false
  }
}

function selectStatus(status: StatusFilter) {
  if (statusFilter.value === status) return
  statusFilter.value = status
  loadGames()
}

await loadGames()

// Delete confirmation, same modal pattern as the user detail's deactivate dialog.
const pendingDelete = ref<AdminUnfinishedGame | null>(null)
const isConfirmOpen = computed(() => pendingDelete.value !== null)
const confirmDialogRef = ref<HTMLElement | null>(null)
const isDeleting = ref(false)
const deleteError = ref('')

function askDelete(game: AdminUnfinishedGame) {
  deleteError.value = ''
  pendingDelete.value = game
}

function cancelDelete() {
  pendingDelete.value = null
}

useModalA11y(isConfirmOpen, confirmDialogRef, cancelDelete)

async function confirmDelete() {
  const game = pendingDelete.value
  if (!game) return
  isDeleting.value = true
  deleteError.value = ''
  try {
    await deleteUnfinishedGame(game.id)
    games.value = games.value.filter(g => g.id !== game.id)
    pendingDelete.value = null
  } catch (err) {
    deleteError.value = adminGameError(err)
    // Already gone or already finished: either way it no longer belongs in this list.
    const status = apiErrorStatus(err)
    if (status === 404 || status === 409) {
      games.value = games.value.filter(g => g.id !== game.id)
    }
    pendingDelete.value = null
  } finally {
    isDeleting.value = false
  }
}

const minuteMs = 60 * 1000
const hourMs = 60 * minuteMs
const dayMs = 24 * hourMs

/** "3 days ago"-style age, the main hint that a game was abandoned. */
function ageLabel(iso: string): string {
  const elapsed = Date.now() - new Date(iso).getTime()
  const rtf = new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' })
  if (elapsed >= dayMs) return rtf.format(-Math.floor(elapsed / dayMs), 'day')
  if (elapsed >= hourMs) return rtf.format(-Math.floor(elapsed / hourMs), 'hour')
  return rtf.format(-Math.max(1, Math.floor(elapsed / minuteMs)), 'minute')
}

function playersLabel(game: AdminUnfinishedGame): string {
  if (!game.players.length) return t('admin.games.noPlayers')
  return game.players.map(p => p.username).join(', ')
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <NuxtLink to="/admin" class="w-fit text-[13px]" style="color: var(--accent-link);">
      {{ $t('admin.games.back') }}
    </NuxtLink>

    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('admin.games.title') }}</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('admin.games.subtitle') }}</p>
    </section>

    <div class="flex flex-wrap gap-2" role="group" :aria-label="$t('admin.games.filterLabel')">
      <button
        v-for="status in statusFilters"
        :key="status"
        type="button"
        :aria-pressed="statusFilter === status"
        class="rounded-full border px-4 py-1.5 text-[13px] transition-colors"
        :style="statusFilter === status
          ? 'border-color: var(--accent-link); color: var(--text); background: rgba(139,92,246,0.15);'
          : 'border-color: var(--input-border); color: var(--text-muted);'"
        @click="selectStatus(status)"
      >
        {{ $t(`admin.games.filters.${status}`) }}
      </button>
    </div>

    <p v-if="deleteError" class="text-sm" style="color: var(--lose);" role="alert">{{ deleteError }}</p>

    <p v-if="loadError" class="text-sm" style="color: var(--lose);">{{ $t('admin.games.loadError') }}</p>
    <p v-else-if="!isLoading && !games.length" class="text-sm" style="color: var(--text-muted);">
      {{ $t('admin.games.empty') }}
    </p>

    <ul v-else class="flex flex-col gap-2">
      <li
        v-for="game in games"
        :key="game.id"
        class="flex flex-wrap items-center justify-between gap-3 rounded-[var(--radius-xl)] border px-5 py-3.5"
        style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
      >
        <div class="flex min-w-0 flex-col gap-0.5">
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="rounded-full px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide"
              :style="game.status === 'active'
                ? 'background: var(--warn-bg); color: var(--warn);'
                : 'background: rgba(139,92,246,0.15); color: #a78bfa;'"
            >
              {{ $t(`admin.games.status.${game.status}`) }}
            </span>
            <span class="text-sm font-semibold">{{ game.playgroup_name ?? $t('admin.games.noPlaygroup') }}</span>
          </div>
          <span class="truncate text-[13px]" style="color: var(--text-dim);">{{ playersLabel(game) }}</span>
        </div>

        <div class="flex items-center gap-4">
          <span class="text-[12px]" style="color: var(--text-dim);" :title="d(new Date(game.created_at), 'short')">
            {{ $t('admin.games.createdAgo', { age: ageLabel(game.created_at) }) }}
          </span>
          <button
            type="button"
            class="rounded-full border px-4 py-1.5 text-[13px] font-semibold"
            style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
            @click="askDelete(game)"
          >
            {{ $t('admin.games.delete') }}
          </button>
        </div>
      </li>
    </ul>

    <button
      v-if="cursor"
      type="button"
      :disabled="isLoadingMore"
      class="self-center rounded-full border px-5 py-2 text-sm disabled:opacity-50"
      style="border-color: var(--input-border); color: var(--text);"
      @click="loadMore"
    >
      {{ isLoadingMore ? t('admin.users.loadingMore') : t('admin.users.loadMore') }}
    </button>

    <div
      v-if="pendingDelete"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="cancelDelete"
    >
      <div
        ref="confirmDialogRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="admin-delete-game-title"
        class="w-full max-w-sm rounded-[var(--radius-xl)] border p-6"
        style="border-color: var(--card-border); background: var(--page-solid);"
      >
        <h2 id="admin-delete-game-title" class="text-[15px] font-medium">
          {{ $t('admin.games.deleteConfirmTitle') }}
        </h2>
        <p class="mt-2 text-[13px]" style="color: var(--text-muted);">
          {{ pendingDelete.playgroup_name ?? $t('admin.games.noPlaygroup') }} · {{ playersLabel(pendingDelete) }}
        </p>
        <p class="mt-2 text-[13px]" style="color: var(--text-muted);">{{ $t('admin.games.deleteConfirmBody') }}</p>

        <div class="mt-5 flex justify-end gap-3">
          <button
            type="button"
            class="rounded-full border px-4 py-2 text-sm"
            style="border-color: var(--input-border); color: var(--text);"
            @click="cancelDelete"
          >
            {{ $t('common.cancel') }}
          </button>
          <button
            type="button"
            :disabled="isDeleting"
            class="rounded-full border px-5 py-2 text-sm font-semibold disabled:opacity-50"
            style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
            @click="confirmDelete"
          >
            {{ $t('admin.games.delete') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
