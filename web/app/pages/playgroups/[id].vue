<script setup lang="ts">
import type { Game, Playgroup, PlaygroupStats, UserSearchResult } from '~/types/api'

const route = useRoute()
const playgroupId = route.params.id as string

const { getPlaygroup, updatePlaygroup, addMember } = usePlaygroups()
const { playgroupStats } = useStatistics()
const { listPlaygroupGames } = useGames()
const { searchUsers } = useUsers()

const { data: playgroup, refresh, error: loadError } = await useAsyncData<Playgroup | null>(
  `playgroup-${playgroupId}`,
  () => getPlaygroup(playgroupId),
  { default: () => null },
)

// Las stats son best-effort: un grupo recién creado, sin partidas todavía, no
// tiene fila en playgroup_statistics_summary y el endpoint puede devolver 404.
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

/** user_id -> username, para no mostrar UUIDs crudos en stats ni en el historial. */
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
  return iso ? new Date(iso).toLocaleDateString() : null
}

function gamePlayerNames(game: Game): string {
  const names = (game.players ?? []).map((p) => usernameFor(p.user_id))
  return names.length ? names.join(', ') : 'Sin jugadores'
}

// -------------------------------------------------------------- renombrar
const editedName = ref(playgroup.value?.name ?? '')
const renameError = ref('')
const renameSuccess = ref(false)
const isRenaming = ref(false)

async function handleRename() {
  renameError.value = ''
  renameSuccess.value = false
  isRenaming.value = true
  try {
    await updatePlaygroup(playgroupId, editedName.value)
    renameSuccess.value = true
    await refresh()
  } catch (err) {
    renameError.value = updatePlaygroupError(err)
  } finally {
    isRenaming.value = false
  }
}

// ------------------------------------------------------------ miembros
const isAddMemberOpen = ref(false)
const addError = ref('')
const addSuccess = ref(false)
const isAdding = ref(false)

// ------------------------------------------------------- busqueda de usuarios
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
  addSuccess.value = false
  memberQuery.value = ''
  searchResults.value = []
  selectedUser.value = null
}

async function handleAddMember() {
  if (!selectedUser.value) return
  addError.value = ''
  addSuccess.value = false
  isAdding.value = true
  try {
    await addMember(playgroupId, selectedUser.value.id)
    memberQuery.value = ''
    selectedUser.value = null
    addSuccess.value = true
    await refresh()
  } catch (err) {
    addError.value = addMemberError(err)
  } finally {
    isAdding.value = false
  }
}
</script>

<template>
  <div class="space-y-8">
    <NuxtLink to="/playgroups" class="text-sm text-indigo-400 hover:text-indigo-300">
      ← Grupos
    </NuxtLink>

    <p v-if="loadError" class="text-sm text-red-400">{{ getPlaygroupError(loadError) }}</p>

    <template v-else-if="playgroup">
      <section>
        <h1 class="text-2xl font-semibold">Editar grupo</h1>
      </section>

      <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <h2 class="font-medium">Nombre</h2>

        <form class="mt-4 flex flex-col gap-3 sm:flex-row" @submit.prevent="handleRename">
          <input
            id="playgroup-edit-name"
            v-model="editedName"
            type="text"
            required
            class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >
          <button
            type="submit"
            :disabled="isRenaming"
            class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
          >
            {{ isRenaming ? 'Guardando…' : 'Guardar' }}
          </button>
        </form>

        <p v-if="renameError" class="mt-3 text-sm text-red-400">{{ renameError }}</p>
        <p v-if="renameSuccess" class="mt-3 text-sm text-emerald-400">Nombre actualizado.</p>
      </section>

      <section v-if="stats">
        <h2 class="font-medium">Estadísticas del grupo</h2>
        <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
          <StatCard label="Partidas jugadas" :value="stats.games_played" />
        </div>
        <ul v-if="stats.members.length" class="mt-3 space-y-2">
          <li
            v-for="member in stats.members"
            :key="member.user_id"
            class="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-sm"
          >
            <span class="text-slate-400">{{ usernameFor(member.user_id) }}</span>
            <span>{{ member.games_won }} / {{ member.games_played }} ganadas</span>
          </li>
        </ul>
      </section>

      <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <div class="flex items-center justify-between">
          <h2 class="font-medium">Miembros</h2>
          <button
            type="button"
            aria-label="Agregar usuario"
            title="Agregar usuario"
            class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100"
            @click="toggleAddMember"
          >
            ➕
          </button>
        </div>

        <ul v-if="playgroup.members?.length" class="mt-3 space-y-2">
          <li
            v-for="member in playgroup.members"
            :key="member.user_id"
            class="rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-sm text-slate-300"
          >
            {{ member.username }}
          </li>
        </ul>
        <p v-else class="mt-3 text-sm text-slate-400">Sin miembros todavía.</p>

        <div v-if="isAddMemberOpen" class="mt-4">
          <form class="flex flex-col gap-3 sm:flex-row" @submit.prevent="handleAddMember">
            <div class="relative flex-1">
              <input
                id="new-member-query"
                v-model="memberQuery"
                type="text"
                required
                autofocus
                autocomplete="off"
                placeholder="Buscar por username o email exacto"
                class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
                @input="onQueryInput"
              >
              <ul
                v-if="searchResults.length"
                class="absolute z-10 mt-1 w-full space-y-1 rounded-lg border border-slate-700 bg-slate-900 p-1 shadow-lg"
              >
                <li v-for="user in searchResults" :key="user.id">
                  <button
                    type="button"
                    class="w-full rounded-md px-2 py-1.5 text-left text-sm text-slate-200 hover:bg-slate-800"
                    @click="selectUser(user)"
                  >
                    {{ user.username }}
                  </button>
                </li>
              </ul>
            </div>
            <button
              type="submit"
              :disabled="isAdding || !selectedUser"
              class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
            >
              {{ isAdding ? 'Agregando…' : 'Agregar' }}
            </button>
          </form>
          <p v-if="isSearching" class="mt-2 text-xs text-slate-500">Buscando…</p>
          <p v-if="searchError" class="mt-2 text-xs text-red-400">{{ searchError }}</p>
        </div>

        <p v-if="addError" class="mt-3 text-sm text-red-400">{{ addError }}</p>
        <p v-if="addSuccess" class="mt-3 text-sm text-emerald-400">Miembro agregado.</p>
      </section>

      <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <h2 class="font-medium">Historial de partidas</h2>

        <p v-if="gamesError" class="mt-3 text-sm text-red-400">
          {{ listPlaygroupGamesError(gamesError) }}
        </p>
        <p v-else-if="!games?.length" class="mt-3 text-sm text-slate-400">
          Todavía no se jugó ninguna partida en este grupo.
        </p>

        <ul v-else class="mt-3 space-y-2">
          <li
            v-for="game in games"
            :key="game.id"
            class="rounded-lg border border-slate-800 bg-slate-900/40 p-3 text-sm"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span
                :class="{
                  'text-emerald-400': game.status === 'finished',
                  'text-amber-400': game.status === 'active',
                  'text-slate-400': game.status === 'pending',
                }"
              >
                {{ gameStatusLabel(game.status) }}
              </span>
              <span v-if="gameDate(game)" class="text-slate-500">{{ gameDate(game) }}</span>
            </div>
            <p class="mt-1 text-slate-400">{{ gamePlayerNames(game) }}</p>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>
