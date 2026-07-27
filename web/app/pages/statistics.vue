<script setup lang="ts">
import type { Deck, DeckStats, UserStats } from '~/types/api'

const { userStats, deckStats } = useStatistics()
const { listDecks } = useDecks()

interface DeckWithStats {
  deck: Deck
  stats: DeckStats | null
}

const { data, error } = await useAsyncData('statistics', async () => {
  const [user, decks] = await Promise.all([userStats(), listDecks()])

  // Las stats por deck se piden solo si el usuario tiene decks propios.
  const perDeck: DeckWithStats[] = await Promise.all(
    decks.map(async (deck) => ({
      deck,
      stats: await deckStats(deck.id).catch(() => null),
    })),
  )

  return { user, perDeck } as { user: UserStats; perDeck: DeckWithStats[] }
})
</script>

<template>
  <div class="space-y-8">
    <section>
      <h1 class="text-2xl font-semibold">Estadísticas</h1>
      <p class="mt-1 text-sm text-slate-400">
        Se recalculan al finalizar cada partida.
      </p>
    </section>

    <p v-if="error" class="text-sm text-red-400">
      No se pudieron cargar las estadísticas.
    </p>

    <template v-else-if="data">
      <section>
        <h2 class="font-medium">Globales</h2>
        <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
          <StatCard label="Partidas" :value="data.user.games_played" />
          <StatCard label="Victorias" :value="data.user.games_won" />
          <StatCard
            label="Win rate"
            :value="winRate(data.user.games_played, data.user.games_won)"
          />
          <StatCard label="Daño total" :value="data.user.total_damage_dealt" />
          <StatCard
            label="Daño de comandante"
            :value="data.user.total_commander_damage_dealt"
          />
          <StatCard label="Eliminaciones" :value="data.user.total_eliminations" />
        </div>
      </section>

      <section>
        <h2 class="font-medium">Por deck</h2>

        <p v-if="!data.perDeck.length" class="mt-3 text-sm text-slate-400">
          Todavía no tenés decks.
          <NuxtLink to="/decks" class="text-indigo-400 hover:text-indigo-300">
            Importá uno desde Moxfield
          </NuxtLink>
          para ver sus estadísticas.
        </p>

        <div v-else class="mt-3 space-y-4">
          <article
            v-for="entry in data.perDeck"
            :key="entry.deck.id"
            class="rounded-xl border border-slate-800 bg-slate-900/60 p-5"
          >
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h3 class="font-medium">{{ entry.deck.name }}</h3>
              <span class="text-sm text-slate-400">{{ entry.deck.commander }}</span>
            </div>

            <p v-if="!entry.stats" class="mt-3 text-sm text-slate-500">
              Sin estadísticas disponibles para este deck.
            </p>

            <div v-else class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
              <StatCard label="Partidas" :value="entry.stats.games_played" />
              <StatCard label="Victorias" :value="entry.stats.games_won" />
              <StatCard
                label="Vida máxima"
                :value="entry.stats.highest_life_total_achieved"
              />
              <StatCard
                label="Daño de comandante"
                :value="entry.stats.total_commander_damage_dealt"
              />
            </div>
          </article>
        </div>
      </section>
    </template>
  </div>
</template>
