<script setup lang="ts">
import type { Game, Playgroup, PlaygroupStats, UserSearchResult } from '~/types/api'

const route = useRoute()
const playgroupId = route.params.id as string
const { t, d } = useI18n()

const { getPlaygroup, updatePlaygroup, addMember } = usePlaygroups()
const { playgroupStats } = useStatistics()
const { listPlaygroupGames } = useGames()
const { searchUsers } = useUsers()
const { showToast } = useToast()

const { data: playgroup, refresh, error: loadError } = await useAsyncData<Playgroup | null>(
  `playgroup-${playgroupId}`,
  () => getPlaygroup(playgroupId),
  { default: () => null },
)

// Stats are best-effort: a freshly created group, with no games yet, doesn't
// have a row in playgroup_statistics_summary and the endpoint may return 404.
const { data: stats } = await useAsyncData<PlaygroupStats | null>(
  `playgroup-stats-${playgroupId}`,
  () => playgroupStats(playgroupId).catch(() => null),
  { default: () => null },
)

const { data: games, error: gamesError } = await useAsyncData<Game[]>(
  `playgroup-games-${playgroupId}`,
  () => listPlaygroupGames(playgroupId),
  { default: () => [] },
)

/** user_id -> username, to avoid showing raw UUIDs in stats or in the history. */
const usernameByUserId = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const member of playgroup.value?.members ?? []) map[member.user_id] = member.username
  return map
})

function usernameFor(userId: string): string {
  return usernameByUserId.value[userId] ?? userId
}

function gameDate(game: Game): string | null {
  const iso = game.finished_at ?? game.started_at
  return iso ? d(new Date(iso), 'short') : null
}

function gamePlayerNames(game: Game): string {
  const names = (game.players ?? []).map((p) => usernameFor(p.user_id))
  return names.length ? names.join(', ') : t('playgroups.detail.history.noPlayers')
}

const rankedMembers = computed(() => {
  if (!stats.value) return []
  return [...stats.value.members]
    .sort((a, b) => (b.games_won / (b.games_played || 1)) - (a.games_won / (a.games_played || 1)))
    .map((m, i) => ({ ...m, rank: i + 1, username: usernameFor(m.user_id) }))
})

// -------------------------------------------------------------- rename
const isRenaming_ = ref(false) // opens/closes the inline input next to the title
const editedName = ref(playgroup.value?.name ?? '')
const renameError = ref('')
const isRenaming = ref(false)

function startRename() {
  editedName.value = playgroup.value?.name ?? ''
  renameError.value = ''
  isRenaming_.value = true
}

async function handleRename() {
  renameError.value = ''
  isRenaming.value = true
  try {
    await updatePlaygroup(playgroupId, editedName.value)
    isRenaming_.value = false
    await refresh()
    showToast(t('toast.groupRenamed'))
  } catch (err) {
    renameError.value = updatePlaygroupError(err)
  } finally {
    isRenaming.value = false
  }
}

// ------------------------------------------------------------ members
const isAddMemberOpen = ref(false)
const addError = ref('')
const isAdding = ref(false)

// ------------------------------------------------------- user search
const memberQuery = ref('')
const searchResults = ref<UserSearchResult[]>([])
const isSearching = ref(false)
const searchError = ref('')
const selectedUser = ref<UserSearchResult | null>(null)
let searchDebounce: ReturnType<typeof setTimeout> | undefined

const existingMemberIds = computed(() => new Set((playgroup.value?.members ?? []).map((m) => m.user_id)))

function onQueryInput() {
  selectedUser.value = null
  searchError.value = ''
  clearTimeout(searchDebounce)
  const query = memberQuery.value.trim()
  if (query.length < 2) {
    searchResults.value = []
    return
  }
  searchDebounce = setTimeout(async () => {
    isSearching.value = true
    try {
      const results = await searchUsers(query)
      searchResults.value = results.filter((u) => !existingMemberIds.value.has(u.id))
    } catch (err) {
      searchError.value = searchUsersError(err)
      searchResults.value = []
    } finally {
      isSearching.value = false
    }
  }, 300)
}

function selectUser(user: UserSearchResult) {
  selectedUser.value = user
  memberQuery.value = user.username
  searchResults.value = []
}

function toggleAddMember() {
  isAddMemberOpen.value = !isAddMemberOpen.value
  addError.value = ''
  memberQuery.value = ''
  searchResults.value = []
  selectedUser.value = null
}

async function handleAddMember() {
  if (!selectedUser.value) return
  addError.value = ''
  isAdding.value = true
  try {
    const added = await addMember(playgroupId, selectedUser.value.id)
    memberQuery.value = ''
    selectedUser.value = null
    isAddMemberOpen.value = false
    await refresh()
    showToast(t('toast.memberAdded', { username: added.username }))
  } catch (err) {
    addError.value = addMemberError(err)
  } finally {
    isAdding.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <NuxtLink to="/playgroups" class="self-start text-[13px]" style="color: var(--accent-link);">{{ $t('playgroups.detail.back') }}</NuxtLink>

    <p v-if="loadError" class="text-sm" style="color: var(--lose);">{{ getPlaygroupError(loadError) }}</p>

    <template v-else-if="playgroup">
      <section>
        <div v-if="!isRenaming_" class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-semibold sm:text-[26px]">{{ playgroup.name }}</h1>
          <button type="button" class="text-sm" style="color: var(--accent-link);" @click="startRename">
            {{ $t('playgroups.detail.rename') }}
          </button>
        </div>
        <form v-else class="flex flex-wrap gap-2.5" @submit.prevent="handleRename">
          <input
            v-model="editedName"
            type="text"
            required
            autofocus
            class="min-w-[220px] flex-1 rounded-full border px-4 py-2 text-sm outline-none"
            style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
          >
          <button
            type="submit"
            :disabled="isRenaming"
            class="rounded-full px-4 py-2 text-sm font-semibold text-[#0a0714] disabled:opacity-50"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          >
            {{ isRenaming ? $t('common.saving') : $t('common.save') }}
          </button>
          <button
            type="button"
            class="rounded-full border px-4 py-2 text-sm"
            style="border-color: var(--input-border); color: var(--text-muted);"
            @click="isRenaming_ = false"
          >
            {{ $t('common.cancel') }}
          </button>
        </form>
        <p v-if="renameError" class="mt-2 text-sm" style="color: var(--lose);">{{ renameError }}</p>
        <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('playgroups.detail.memberCount', playgroup.members?.length ?? 0) }}</p>
      </section>

      <section class="flex flex-wrap gap-3.5">
        <StatCard
          :label="$t('playgroups.detail.stats.gamesPlayed')"
          :value="stats?.games_played ?? 0"
          tint="rgba(139,92,246,0.18)"
          class="min-w-[180px] flex-1"
        />
        <StatCard
          :label="$t('playgroups.detail.stats.members')"
          :value="playgroup.members?.length ?? 0"
          tint="rgba(168,85,247,0.15)"
          class="min-w-[180px] flex-1"
        />
      </section>

      <section>
        <div class="mb-3.5 flex items-baseline justify-between">
          <h2 class="text-[15px] font-medium">{{ $t('playgroups.detail.ranking.heading') }}</h2>
          <button type="button" class="text-[13px]" style="color: var(--accent-link);" @click="toggleAddMember">
            {{ isAddMemberOpen ? $t('playgroups.detail.ranking.close') : $t('playgroups.detail.ranking.addMember') }}
          </button>
        </div>

        <div
          v-if="isAddMemberOpen"
          class="mb-3.5 flex flex-col gap-2.5 rounded-[22px] border p-4"
          style="border-color: var(--card-border); background: var(--card-bg-strong);"
        >
          <div class="relative">
            <input
              v-model="memberQuery"
              type="text"
              required
              autofocus
              autocomplete="off"
              :placeholder="$t('playgroups.detail.ranking.searchPlaceholder')"
              class="w-full rounded-full border px-4 py-2.5 text-[13px] outline-none"
              style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
              @input="onQueryInput"
            >
            <ul
              v-if="searchResults.length"
              class="absolute z-10 mt-1 w-full space-y-1 rounded-2xl border p-1 shadow-lg"
              style="border-color: var(--card-border); background: var(--page-solid);"
            >
              <li v-for="user in searchResults" :key="user.id">
                <button
                  type="button"
                  class="w-full rounded-xl px-3 py-2 text-left text-sm hover:bg-white/5"
                  style="color: var(--text);"
                  @click="selectUser(user)"
                >
                  {{ user.username }}
                </button>
              </li>
            </ul>
          </div>
          <form class="flex gap-2.5" @submit.prevent="handleAddMember">
            <button
              type="submit"
              :disabled="isAdding || !selectedUser"
              class="rounded-full px-5 py-2 text-[13px] font-semibold text-[#0a0714] disabled:opacity-50"
              style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            >
              {{ isAdding ? $t('playgroups.detail.ranking.adding') : $t('playgroups.detail.ranking.add') }}
            </button>
          </form>
          <p v-if="isSearching" class="text-xs" style="color: var(--text-dim);">{{ $t('playgroups.detail.ranking.searching') }}</p>
          <p v-if="searchError" class="text-xs" style="color: var(--lose);">{{ searchError }}</p>
          <p v-if="addError" class="text-xs" style="color: var(--lose);">{{ addError }}</p>
        </div>

        <div class="overflow-hidden rounded-3xl border" style="border-color: var(--card-border);">
          <div
            class="grid grid-cols-[32px_1fr_90px] gap-2 px-5 py-3 text-[11px] uppercase tracking-wide sm:grid-cols-[32px_1fr_90px_90px_90px]"
            style="background: rgba(255,255,255,0.05); color: var(--text-dim);"
          >
            <span>#</span><span>{{ $t('playgroups.detail.ranking.columns.player') }}</span>
            <span class="hidden sm:inline">{{ $t('playgroups.detail.ranking.columns.games') }}</span><span class="hidden sm:inline">{{ $t('playgroups.detail.ranking.columns.wins') }}</span>
            <span>{{ $t('playgroups.detail.ranking.columns.winRate') }}</span>
          </div>
          <p v-if="!rankedMembers.length" class="px-5 py-4 text-sm" style="color: var(--text-muted);">
            {{ $t('playgroups.detail.ranking.empty') }}
          </p>
          <div
            v-for="m in rankedMembers"
            :key="m.user_id"
            class="grid grid-cols-[32px_1fr_90px] items-center gap-2 border-t px-5 py-3.5 sm:grid-cols-[32px_1fr_90px_90px_90px]"
            style="border-color: rgba(255,255,255,0.06); background: rgba(255,255,255,0.02);"
          >
            <span class="text-[13px]" style="color: var(--text-dim);">{{ m.rank }}</span>
            <span class="flex items-center gap-2 text-sm">
              <span class="inline-block h-[22px] w-[22px] flex-shrink-0 rounded-full" :style="{ background: avatarColor(m.rank - 1) }" />
              {{ m.username }}
            </span>
            <span class="hidden text-[13px] sm:inline" style="color: var(--text-muted);">{{ m.games_played }}</span>
            <span class="hidden text-[13px] sm:inline" style="color: var(--text-muted);">{{ m.games_won }}</span>
            <span class="text-[13px] font-semibold" style="color: var(--accent-link);">
              {{ winRate(m.games_played, m.games_won) }}
            </span>
          </div>
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('playgroups.detail.members.heading') }}</h2>
        <div class="flex flex-wrap gap-2">
          <span
            v-for="member in playgroup.members"
            :key="member.user_id"
            class="rounded-full border px-3.5 py-1.5 text-sm"
            style="border-color: var(--card-border); background: var(--card-bg); color: var(--text-muted);"
          >
            {{ member.username }}
          </span>
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('playgroups.detail.history.heading') }}</h2>

        <p v-if="gamesError" class="text-sm" style="color: var(--lose);">
          {{ listPlaygroupGamesError(gamesError) }}
        </p>
        <p v-else-if="!games?.length" class="text-sm" style="color: var(--text-muted);">
          {{ $t('playgroups.detail.history.empty') }}
        </p>

        <div v-else class="flex flex-col gap-2.5">
          <div
            v-for="game in games"
            :key="game.id"
            class="rounded-[20px] border px-5 py-3"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span
                class="text-sm"
                :style="{
                  color: game.status === 'finished' ? 'var(--win)' : game.status === 'active' ? '#fbbf24' : 'var(--text-muted)',
                }"
              >
                {{ gameStatusLabel(game.status) }}
              </span>
              <span v-if="gameDate(game)" class="text-[13px]" style="color: var(--text-dim);">{{ gameDate(game) }}</span>
            </div>
            <p class="mt-1 text-[13px]" style="color: var(--text-muted);">{{ gamePlayerNames(game) }}</p>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>
