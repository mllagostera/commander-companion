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

  // games_played is best-effort: a group that never played may return 404
  // (see GetPlaygroupStats in internal/statistics/service.go).
  const groupsSummary: GroupSummary[] = await Promise.all(
    playgroups.map(async (playgroup) => ({
      ...playgroup,
      memberCount: playgroup.members?.length ?? 0,
      gamesPlayed: await playgroupStats(playgroup.id).then((s) => s.games_played).catch(() => 0),
    })),
  )

  return { user, perDeck, groupsSummary } as { user: UserStats; perDeck: DeckWithStats[]; groupsSummary: GroupSummary[] }
})

// Same reason as in the dashboard: without this refresh, useAsyncData reuses the cached payload from
// a previous visit instead of reflecting the recalculation the backend triggers when a game finishes.
onMounted(() => refresh())
</script>

<template>
  <div class="flex flex-col gap-9">
    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('statistics.title') }}</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('statistics.subtitle') }}</p>
    </section>

    <p v-if="error" class="text-sm" style="color: var(--lose);">{{ $t('statistics.loadError') }}</p>

    <template v-else-if="data">
      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('statistics.global.heading') }}</h2>
        <div class="mb-3.5 flex flex-wrap gap-3.5">
          <div class="flex min-w-[220px] flex-1 items-center gap-4 rounded-[24px] border p-5" style="border-color: var(--card-border); background: var(--card-bg);">
            <WinRateRing :played="data.user.games_played" :won="data.user.games_won" />
            <div>
              <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('statistics.global.winLoss') }}</p>
              <p class="mt-1.5 text-[13px]" style="color: var(--text);">
                {{ $t('statistics.global.wonLost', { won: data.user.games_won, lost: data.user.games_played - data.user.games_won }) }}
              </p>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap gap-3.5">
          <StatCard :label="$t('statistics.global.games')" :value="data.user.games_played" class="min-w-[150px] flex-1" />
          <StatCard :label="$t('statistics.global.wins')" :value="data.user.games_won" value-color="var(--win)" class="min-w-[150px] flex-1" />
          <StatCard :label="$t('statistics.global.winRate')" :value="winRate(data.user.games_played, data.user.games_won)" value-color="#e9b8fb" class="min-w-[150px] flex-1" />
          <StatCard :label="$t('statistics.global.totalDamage')" :value="data.user.total_damage_dealt" class="min-w-[150px] flex-1" />
          <StatCard :label="$t('statistics.global.commanderDamage')" :value="data.user.total_commander_damage_dealt" class="min-w-[150px] flex-1" />
          <StatCard :label="$t('statistics.global.eliminations')" :value="data.user.total_eliminations" class="min-w-[150px] flex-1" />
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('statistics.perDeck.heading') }}</h2>

        <p v-if="!data.perDeck.length" class="text-sm" style="color: var(--text-muted);">
          {{ $t('statistics.perDeck.noDecksIntro') }}
          <NuxtLink to="/decks" style="color: var(--accent-link);">{{ $t('statistics.perDeck.importLink') }}</NuxtLink>
          {{ $t('statistics.perDeck.noDecksOutro') }}
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
                {{ $t('statistics.perDeck.noStats') }}
              </p>

              <div v-else class="mt-3.5 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.perDeck.games') }}</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.games_played }}</p>
                </div>
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.perDeck.wins') }}</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.games_won }}</p>
                </div>
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.perDeck.maxLife') }}</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.highest_life_total_achieved }}</p>
                </div>
                <div>
                  <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.perDeck.commanderDamage') }}</p>
                  <p class="mt-1 text-lg font-semibold">{{ entry.stats.total_commander_damage_dealt }}</p>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('statistics.perGroup.heading') }}</h2>
        <p v-if="!data.groupsSummary.length" class="text-sm" style="color: var(--text-muted);">
          {{ $t('statistics.perGroup.empty') }}
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
            <span class="text-[13px]" style="color: var(--text-dim);">{{ $t('statistics.perGroup.summary', { games: g.gamesPlayed, members: g.memberCount }) }}</span>
          </NuxtLink>
        </div>
      </section>
    </template>
  </div>
</template>
