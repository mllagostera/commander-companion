<script setup lang="ts">
import type { Deck, DeckStats, Game, Playgroup } from '~/types/api'

const { d } = useI18n()
const { user } = useAuth()
const { userStats, deckStats } = useStatistics()
const { listDecks } = useDecks()
const { listPlaygroups } = usePlaygroups()
const { listPlaygroupGames } = useGames()

interface RecentGame { game: Game; groupName: string; won: boolean }
interface GroupSummary extends Playgroup { memberCount: number; gamesPlayed: number }
interface DeckWithStats { deck: Deck; stats: DeckStats | null }

const { data, error, refresh } = await useAsyncData('dashboard', async () => {
  const [stats, decks, playgroups] = await Promise.all([userStats(), listDecks(), listPlaygroups()])

  const groupsWithGames = await Promise.all(
    playgroups.map(async (playgroup) => ({
      playgroup,
      // Best-effort: si el historial falla para un grupo no tumba el resto del dashboard.
      games: await listPlaygroupGames(playgroup.id).catch(() => [] as Game[]),
    })),
  )

  const groups: GroupSummary[] = groupsWithGames.map(({ playgroup, games }) => ({
    ...playgroup,
    memberCount: playgroup.members?.length ?? 0,
    gamesPlayed: games.filter((g) => g.status === 'finished').length,
  }))

  const userId = user.value?.id
  const recentGames: RecentGame[] = groupsWithGames
    .flatMap(({ playgroup, games }) =>
      games
        .filter((g) => g.status === 'finished')
        .map((g) => {
          const me = g.players?.find((p) => p.user_id === userId)
          return { game: g, groupName: playgroup.name, won: !!me && !me.is_eliminated }
        }),
    )
    .sort((a, b) => {
      const at = new Date(a.game.finished_at ?? a.game.started_at ?? 0).getTime()
      const bt = new Date(b.game.finished_at ?? b.game.started_at ?? 0).getTime()
      return bt - at
    })

  // Racha actual: resultados consecutivos iguales desde la partida más reciente.
  let streak = 0
  if (recentGames.length) {
    const latestResult = recentGames[0]!.won
    for (const rg of recentGames) {
      if (rg.won !== latestResult) break
      streak++
    }
  }

  const deckEntries: DeckWithStats[] = await Promise.all(
    decks.map(async (deck) => ({ deck, stats: await deckStats(deck.id).catch(() => null) })),
  )
  const bestDeckEntry = [...deckEntries]
    .filter((e) => e.stats && e.stats.games_played > 0)
    .sort((a, b) => (b.stats!.games_won / b.stats!.games_played) - (a.stats!.games_won / a.stats!.games_played))[0]
    ?? null
  const dashboardDecks = [...deckEntries]
    .sort((a, b) => (b.stats?.games_played ?? 0) - (a.stats?.games_played ?? 0))
    .slice(0, 3)

  return {
    stats,
    decks,
    groups,
    dashboardGroups: groups.slice(0, 3),
    dashboardDecks,
    recentGames: recentGames.slice(0, 3),
    streak,
    streakWon: recentGames[0]?.won ?? null,
    bestDeckEntry,
  }
})

// useAsyncData reusa el payload cacheado al volver a esta página en la misma sesión (p. ej.
// después de terminar una partida en Android): sin este refresh, el resumen queda desactualizado.
onMounted(() => refresh())

function gameDate(game: Game): string {
  const iso = game.finished_at ?? game.started_at
  return iso ? d(new Date(iso), 'short') : '—'
}
</script>

<template>
  <div class="flex flex-col gap-9">
    <section class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <p class="text-[13px]" style="color: var(--text-dim);">{{ $t('dashboard.sessionAs') }}</p>
        <h1 class="mt-1.5 text-[26px] font-semibold sm:text-[30px]">{{ $t('dashboard.greeting', { username: user?.username ?? '…' }) }}</h1>
        <p style="color: var(--text-muted);">{{ user?.email }}</p>
      </div>
      <NuxtLink
        to="/play"
        class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
        style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
      >
        {{ $t('dashboard.newGame') }}
      </NuxtLink>
    </section>

    <p v-if="error" class="text-sm" style="color: var(--lose);">{{ $t('dashboard.loadError') }}</p>

    <template v-else-if="data">
      <section class="flex flex-wrap gap-3.5">
        <StatCard :label="$t('dashboard.stats.games')" :value="data.stats.games_played" tint="rgba(139,92,246,0.22)" class="min-w-[160px] flex-1" />
        <StatCard :label="$t('dashboard.stats.wins')" :value="data.stats.games_won" tint="rgba(196,181,253,0.18)" value-color="var(--win)" class="min-w-[160px] flex-1" />
        <StatCard :label="$t('dashboard.stats.winRate')" :value="winRate(data.stats.games_played, data.stats.games_won)" tint="rgba(168,85,247,0.18)" value-color="#e9b8fb" class="min-w-[160px] flex-1" />
        <StatCard :label="$t('dashboard.stats.decks')" :value="data.decks.length" tint="rgba(216,180,254,0.16)" value-color="#ddd6fe" class="min-w-[160px] flex-1" />
      </section>

      <section class="grid grid-cols-1 gap-3.5 sm:grid-cols-3">
        <div class="flex items-center gap-4 rounded-[24px] border p-5" style="border-color: var(--card-border); background: var(--card-bg);">
          <WinRateRing :played="data.stats.games_played" :won="data.stats.games_won" />
          <div>
            <p class="text-xs" style="color: var(--text-dim);">{{ $t('dashboard.winLoss.heading') }}</p>
            <p class="mt-1 text-[13px]" style="color: var(--text);">{{ $t('dashboard.winLoss.won', { count: data.stats.games_won }) }}</p>
            <p class="text-[13px]" style="color: var(--text-dim);">{{ $t('dashboard.winLoss.lost', { count: data.stats.games_played - data.stats.games_won }) }}</p>
          </div>
        </div>

        <div class="rounded-[20px] border p-5" style="border-color: rgba(196,181,253,0.25); background: linear-gradient(160deg, rgba(196,181,253,0.12), var(--card-bg));">
          <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('dashboard.bestDeck.heading') }}</p>
          <template v-if="data.bestDeckEntry">
            <p class="mt-2 text-base font-semibold">{{ data.bestDeckEntry.deck.name }}</p>
            <p class="mt-1 text-[13px]" style="color: var(--text-muted);">
              {{ $t('dashboard.bestDeck.summary', {
                winRate: winRate(data.bestDeckEntry.stats!.games_played, data.bestDeckEntry.stats!.games_won),
                games: data.bestDeckEntry.stats!.games_played,
              }) }}
            </p>
          </template>
          <p v-else class="mt-2 text-[13px]" style="color: var(--text-muted);">{{ $t('dashboard.bestDeck.empty') }}</p>
        </div>

        <div class="rounded-[26px] border p-5" style="border-color: rgba(168,85,247,0.22); background: linear-gradient(160deg, rgba(168,85,247,0.12), var(--card-bg));">
          <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('dashboard.streak.heading') }}</p>
          <template v-if="data.streak">
            <p class="mt-2 text-base font-semibold" :style="{ color: data.streakWon ? 'var(--win)' : 'var(--lose)' }">
              {{ data.streakWon ? $t('dashboard.streak.wins', { count: data.streak }) : $t('dashboard.streak.losses', { count: data.streak }) }}
            </p>
            <p class="mt-1 text-[13px]" style="color: var(--text-muted);">{{ $t('dashboard.streak.sinceLast') }}</p>
          </template>
          <p v-else class="mt-2 text-[13px]" style="color: var(--text-muted);">{{ $t('dashboard.streak.empty') }}</p>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-6 lg:grid-cols-[1.2fr_1fr]">
        <div>
          <div class="mb-3.5 flex items-baseline justify-between">
            <h2 class="text-[15px] font-medium">{{ $t('dashboard.groups.heading') }}</h2>
            <NuxtLink to="/playgroups" class="text-[13px]" style="color: var(--accent-link);">{{ $t('dashboard.groups.viewAll') }}</NuxtLink>
          </div>
          <div v-if="!data.dashboardGroups.length" class="text-[13px]" style="color: var(--text-muted);">
            {{ $t('dashboard.groups.empty') }}
          </div>
          <div v-else class="flex flex-col gap-2.5">
            <NuxtLink
              v-for="g in data.dashboardGroups"
              :key="g.id"
              :to="`/playgroups/${g.id}`"
              class="flex items-center justify-between rounded-[22px] border px-[18px] py-3.5 transition-all hover:translate-x-1"
              style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
            >
              <span>
                <span class="text-sm font-medium">{{ g.name }}</span>
                <span class="mt-0.5 block text-xs" style="color: var(--text-dim);">
                  {{ $t('dashboard.groups.summary', { members: g.memberCount, games: g.gamesPlayed }) }}
                </span>
              </span>
              <span class="text-[13px]" style="color: var(--accent-link);">→</span>
            </NuxtLink>
          </div>
        </div>

        <div>
          <div class="mb-3.5 flex items-baseline justify-between">
            <h2 class="text-[15px] font-medium">{{ $t('dashboard.decksSection.heading') }}</h2>
            <NuxtLink to="/statistics" class="text-[13px]" style="color: var(--accent-link);">{{ $t('dashboard.decksSection.viewStats') }}</NuxtLink>
          </div>
          <div v-if="!data.dashboardDecks.length" class="text-[13px]" style="color: var(--text-muted);">
            {{ $t('dashboard.decksSection.empty') }}
          </div>
          <div v-else class="grid grid-cols-3 gap-2.5">
            <div v-for="entry in data.dashboardDecks" :key="entry.deck.id" class="relative">
              <DeckArt :deck="entry.deck" />
              <div class="pointer-events-none absolute inset-0 rounded-[18px]" style="background: linear-gradient(180deg, rgba(10,7,20,0.1) 30%, rgba(10,7,20,0.9) 100%);" />
              <p class="pointer-events-none absolute inset-x-0 bottom-0 p-2.5 text-xs font-semibold leading-tight text-white">
                {{ entry.deck.name }}
              </p>
            </div>
          </div>
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('dashboard.recentGames.heading') }}</h2>
        <p v-if="!data.recentGames.length" class="text-[13px]" style="color: var(--text-muted);">
          {{ $t('dashboard.recentGames.empty') }}
        </p>
        <div v-else class="flex flex-col gap-2.5">
          <div
            v-for="rg in data.recentGames"
            :key="rg.game.id"
            class="flex items-center justify-between rounded-[20px] border px-[18px] py-3"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <span class="text-[13px]" style="color: var(--text-muted);">{{ gameDate(rg.game) }} · {{ rg.groupName }}</span>
            <span
              class="rounded-full px-3 py-1 text-xs font-semibold"
              :style="{ background: rg.won ? 'var(--win-bg)' : 'var(--lose-bg)', color: rg.won ? 'var(--win)' : 'var(--lose)' }"
            >
              {{ rg.won ? $t('dashboard.recentGames.won') : $t('dashboard.recentGames.lost') }}
            </span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>
