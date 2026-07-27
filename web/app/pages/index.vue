<script setup lang="ts">
import type { Deck, UserStats } from '~/types/api'

const { user } = useAuth()
const { userStats } = useStatistics()
const { listDecks } = useDecks()

const { data, error } = await useAsyncData('dashboard', async () => {
  const [stats, decks] = await Promise.all([userStats(), listDecks()])
  return { stats, decks } as { stats: UserStats; decks: Deck[] }
})
</script>

<template>
  <div class="space-y-8">
    <section>
      <p class="text-sm text-slate-400">Sesión iniciada como</p>
      <h1 class="mt-1 text-2xl font-semibold">{{ user?.username ?? '…' }}</h1>
      <p class="text-sm text-slate-400">{{ user?.email }}</p>
    </section>

    <p v-if="error" class="text-sm text-red-400">
      No se pudo cargar el resumen.
    </p>

    <template v-else-if="data">
      <section>
        <div class="flex items-baseline justify-between">
          <h2 class="font-medium">Resumen</h2>
          <NuxtLink to="/statistics" class="text-sm text-indigo-400 hover:text-indigo-300">
            Ver todas las estadísticas
          </NuxtLink>
        </div>
        <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="Partidas" :value="data.stats.games_played" />
          <StatCard label="Victorias" :value="data.stats.games_won" />
          <StatCard
            label="Win rate"
            :value="winRate(data.stats.games_played, data.stats.games_won)"
          />
          <StatCard label="Decks" :value="data.decks.length" />
        </div>
      </section>

      <section>
        <div class="flex items-baseline justify-between">
          <h2 class="font-medium">Decks</h2>
          <NuxtLink to="/decks" class="text-sm text-indigo-400 hover:text-indigo-300">
            Importar desde Moxfield
          </NuxtLink>
        </div>

        <p v-if="!data.decks.length" class="mt-3 text-sm text-slate-400">
          Todavía no tenés decks.
        </p>

        <ul v-else class="mt-3 space-y-2">
          <li
            v-for="deck in data.decks.slice(0, 5)"
            :key="deck.id"
            class="rounded-lg border border-slate-800 bg-slate-900/40 px-4 py-3"
          >
            <span class="font-medium">{{ deck.name }}</span>
            <span class="ml-2 text-sm text-slate-400">{{ deck.commander }}</span>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>
