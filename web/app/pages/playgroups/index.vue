<script setup lang="ts">
import type { Playgroup } from '~/types/api'

const { listPlaygroups, createPlaygroup } = usePlaygroups()
const { playgroupStats } = useStatistics()

interface PlaygroupWithCounts extends Playgroup {
  memberCount: number
  gamesPlayed: number
}

const { data: playgroups, refresh, error: listError } = await useAsyncData<PlaygroupWithCounts[]>(
  'playgroups',
  async () => {
    const list = await listPlaygroups()
    // games_played es best-effort: un grupo que nunca jugó puede devolver 404
    // (ver GetPlaygroupStats en internal/statistics/service.go).
    return Promise.all(
      list.map(async (playgroup) => ({
        ...playgroup,
        memberCount: playgroup.members?.length ?? 0,
        gamesPlayed: await playgroupStats(playgroup.id).then((s) => s.games_played).catch(() => 0),
      })),
    )
  },
  { default: () => [] },
)

const isCreateModalOpen = ref(false)
const newName = ref('')
const createError = ref('')
const isCreating = ref(false)

function openCreateModal() {
  newName.value = ''
  createError.value = ''
  isCreateModalOpen.value = true
}

function closeCreateModal() {
  isCreateModalOpen.value = false
}

async function handleCreate() {
  createError.value = ''
  isCreating.value = true
  try {
    const created = await createPlaygroup(newName.value)
    closeCreateModal()
    await refresh()
    await navigateTo(`/playgroups/${created.id}`)
  } catch (err) {
    createError.value = createPlaygroupError(err)
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <div class="space-y-8">
    <section class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Grupos</h1>
        <p class="mt-1 text-sm text-slate-400">
          Los grupos de juego reúnen a los jugadores con los que jugás seguido.
        </p>
      </div>
      <button
        type="button"
        class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400"
        @click="openCreateModal"
      >
        + Crear grupo
      </button>
    </section>

    <section>
      <p v-if="listError" class="text-sm text-red-400">
        No se pudieron cargar los grupos.
      </p>

      <p v-else-if="!playgroups?.length" class="text-sm text-slate-400">
        Todavía no sos miembro de ningún grupo. Creá uno para empezar.
      </p>

      <ul v-else class="space-y-2">
        <li
          v-for="playgroup in playgroups"
          :key="playgroup.id"
          class="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-900/40 p-4"
        >
          <div>
            <p>{{ playgroup.name }}</p>
            <p class="mt-1 text-sm text-slate-400">
              {{ playgroup.memberCount }} {{ playgroup.memberCount === 1 ? 'usuario' : 'usuarios' }}
              ·
              {{ playgroup.gamesPlayed }} {{ playgroup.gamesPlayed === 1 ? 'partida' : 'partidas' }}
            </p>
          </div>
          <NuxtLink
            :to="`/playgroups/${playgroup.id}`"
            aria-label="Editar grupo"
            title="Editar grupo"
            class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-100"
          >
            ⚙️
          </NuxtLink>
        </li>
      </ul>
    </section>

    <div
      v-if="isCreateModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      @click.self="closeCreateModal"
    >
      <div class="w-full max-w-sm rounded-xl border border-slate-800 bg-slate-900 p-6">
        <h2 class="font-medium">Crear grupo</h2>

        <form class="mt-4 space-y-3" @submit.prevent="handleCreate">
          <input
            id="playgroup-name"
            v-model="newName"
            type="text"
            required
            autofocus
            placeholder="Nombre del grupo"
            class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none"
          >

          <p v-if="createError" class="text-sm text-red-400">{{ createError }}</p>

          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="rounded-lg border border-slate-700 px-4 py-2 text-sm hover:bg-slate-800"
              @click="closeCreateModal"
            >
              Cancelar
            </button>
            <button
              type="submit"
              :disabled="isCreating"
              class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-slate-950 hover:bg-indigo-400 disabled:opacity-50"
            >
              {{ isCreating ? 'Creando…' : 'Crear' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
