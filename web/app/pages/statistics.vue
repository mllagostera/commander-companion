<script setup lang="ts">
import type { Deck, DeckStats, Playgroup, UserStats } from '~/types/api'

const { userStats, deckStats, playgroupStats } = useStatistics()
const { listDecks } = useDecks()
const { listPlaygroups } = usePlaygroups()

interface DeckWithStats {
  deck: Deck
  stats: DeckStats | null
}

interface GroupSummary extends Playgroup {
  memberCount: number
  gamesPlayed: number
}

const { data, error, refresh } = await useAsyncData('statistics', async () => {
  const [user, decks, playgroups] = await Promise.all([userStats(), listDecks(), listPlaygroups()])

  const perDeck: DeckWithStats[] = await Promise.all(
    decks.map(async (deck) => ({
      deck,
      stats: await deckStats(deck.id).catch(() => null),
    })),
  )

  // games_played es best-effort: un grupo que nunca jugó puede devolver 404
  // (ver GetPlaygroupStats en internal/statistics/service.go).
  const groupsSummary: GroupSummary[] = await Promise.all(
    playgroups.map(async (playgroup) => ({
      ...playgroup,
      memberCount: playgroup.members?.length ?? 0,
      gamesPlayed: await playgroupStats(playgroup.id).then((s) => s.games_played).catch(() => 0),
    })),
  )

  return { user, perDeck, groupsSummary } as { user: UserStats; perDeck: DeckWithStats[]; groupsSummary: GroupSummary[] }
})

// Mismo motivo que en el dashboard: sin este refresh, useAsyncData reusa el payload cacheado de
// una visita anterior en vez de reflejar el recálculo que dispara el backend al finalizar una partida.
onMounted(() => refresh())
</script>

<template>
  <div class="flex flex-col gap-9">
    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">Estadísticas</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">Se recalculan al finalizar cada partida.</p>
    </section>

    <p v-if="error" class="text-sm" style="color: var(--lose);">No se pudieron cargar las estadísticas.</p>

    <template v-else-if="data">
      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">Globales</h2>
        <div class="mb-3.5 flex flex-wrap gap-3.5">
          <div class="flex min-w-[220px] flex-1 items-center gap-4 rounded-[24px] border p-5" style="border-color: var(--card-border); background: var(--card-bg);">
            <WinRateRing :played="data.user.games_played" :won="data.user.games_won" />
            <div>
              <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">Victorias / derrotas</p>
              <p class="mt-1.5 text-[13px]" style="color: var(--text);">
                {{ data.user.games_won }} ganadas · {{ data.user.games_played - data.user.games_won }} perdidas
              </p>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap gap-3.5">
          <StatCard label="Partidas" :value="data.user.games_played" class="min-w-[150px] flex-1" />
          <StatCard label="Victorias" :value="data.user.games_won" value-color="var(--win)" class="min-w-[150px] flex-1" />
          <StatCard label="Win rate" :value="winRate(data.user.games_played, data.user.games_won)" value-color="#e9b8fb" class="min-w-[150px] flex-1" />
          <StatCard label="Daño total" :value="data.user.total_damage_dealt" class="min-w-[150px] flex-1" />
          <StatCard label="Daño de comandante" :value="data.user.total_commander_damage_dealt" class="min-w-[150px] flex-1" />
          <StatCard label="Eliminaciones" :value="data.user.total_eliminations" class="min-w-[150px] flex-1" />
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">Por deck</h2>

        <p v-if="!data.perDeck.length" class="text-sm" style="color: var(--text-muted);">
          Todavía no tenés decks.
          <NuxtLink to="/decks" style="color: var(--accent-link);">Importá uno desde Moxfield</NuxtLink>
          para ver sus estadísticas.
        </p>

        <div v-else class="flex flex-col gap-3.5">
          <article
            v-for="entry in data.perDeck"
            :key="entry.deck.id"
            class="flex gap-4 rounded-[28px] border p-[22px]"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <DeckArt :deck="entry.deck" class="h-[76px] w-[76px] flex-shrink-0" rounded="rounded-2xl" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-baseline justify-between gap-2">
                <h3 class="text-[15px] font-medium">{{ entry.deck.name }}</h3>
                <span class="text-[13px]" style="color: var(--text-dim);">{{ entry.deck.commander }}</span>
              </div>

              <p v-if="!entry.stats" class="mt-3 text-sm" style="color: var(--text-dim);">
                Sin estadísticas disponibles para este deck.
              </p>

              <div v-else class="mt-3.5 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">Partidas</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.games_played }}</p>
                </div>
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">Victorias</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.games_won }}</p>
                </div>
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">Vida máxima</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.highest_life_total_achieved }}</p>
                </div>
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">Daño comandante</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.total_commander_damage_dealt }}</p>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">Por grupo de juego</h2>
        <p v-if="!data.groupsSummary.length" class="text-sm" style="color: var(--text-muted);">
          Todavía no sos miembro de ningún grupo.
        </p>
        <div v-else class="flex flex-col gap-2.5">
          <NuxtLink
            v-for="g in data.groupsSummary"
            :key="g.id"
            :to="`/playgroups/${g.id}`"
            class="flex items-center justify-between rounded-[20px] border px-5 py-3.5 transition-colors hover:bg-white/[0.03]"
            style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
          >
            <span class="text-sm font-medium">{{ g.name }}</span>
            <span class="text-[13px]" style="color: var(--text-dim);">{{ g.gamesPlayed }} partidas · {{ g.memberCount }} miembros</span>
          </NuxtLink>
        </div>
      </section>
    </template>
  </div>
</template>
