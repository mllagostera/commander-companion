<script setup lang="ts">
import type { Deck, DeckStats, FinishedGame, FinishedGamePlayer, OpponentStats, PlaygroupGameCount, UserStats } from '~/types/api'

const { userStats, allDeckStats, playgroupGameCounts, opponentStats, listFinishedGames } = useStatistics()
const { listAllDecks } = useDecks()
const { d, t } = useI18n()

interface DeckWithStats {
  deck: Deck
  stats: DeckStats | null
}

type DeckSortOrder = 'recent' | 'winRate' | 'gamesPlayed'

const { data, error, refresh } = await useAsyncData('statistics', async () => {
  const [user, decks, deckStatsList, groupCounts, opponents] = await Promise.all([
    userStats(), listAllDecks(), allDeckStats(), playgroupGameCounts(), opponentStats(),
  ])
  const statsByDeckId = new Map(deckStatsList.map((s) => [s.deck_id, s]))
  const perDeck: DeckWithStats[] = decks.map((deck) => ({ deck, stats: statsByDeckId.get(deck.id) ?? null }))

  return { user, perDeck, groupCounts, opponents } as {
    user: UserStats
    perDeck: DeckWithStats[]
    groupCounts: PlaygroupGameCount[]
    opponents: OpponentStats[]
  }
})

// Same reason as in the dashboard: without this refresh, useAsyncData reuses the cached payload from
// a previous visit instead of reflecting the recalculation the backend triggers when a game finishes.
onMounted(() => refresh())

// -------------------------------------------------------------- head-to-head / most-played group

const mostPlayedOpponent = computed(() => {
  const list = data.value?.opponents ?? []
  return list.length ? list.reduce((a, b) => (b.games_together > a.games_together ? b : a)) : null
})

const archenemy = computed(() => {
  const list = (data.value?.opponents ?? []).filter((o) => o.times_eliminated_by_opponent > 0)
  return list.length ? list.reduce((a, b) => (b.times_eliminated_by_opponent > a.times_eliminated_by_opponent ? b : a)) : null
})

const mostPlayedGroup = computed(() => {
  const list = (data.value?.groupCounts ?? []).filter((g) => g.games_played > 0)
  return list.length ? list.reduce((a, b) => (b.games_played > a.games_played ? b : a)) : null
})

// -------------------------------------------------------------------------- tabs + deck sorting

const activeTab = ref<'decks' | 'games'>('decks')

// Left/right arrow switches tabs, per the APG tabs pattern — with only two
// tabs, "switch" and "wrap to the other end" are the same operation.
function handleTabKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  activeTab.value = activeTab.value === 'decks' ? 'games' : 'decks'
  nextTick(() => {
    document.getElementById(activeTab.value === 'decks' ? 'statistics-tab-decks' : 'statistics-tab-games')?.focus()
  })
}

const deckSort = ref<DeckSortOrder>('recent')
const deckSortOptions = computed(() => [
  { value: 'recent', label: t('statistics.perDeck.sort.recent') },
  { value: 'winRate', label: t('statistics.perDeck.sort.winRate') },
  { value: 'gamesPlayed', label: t('statistics.perDeck.sort.gamesPlayed') },
])

function winRateOf(stats: DeckStats | null): number {
  return stats?.games_played ? stats.games_won / stats.games_played : -1
}

const sortedDecks = computed(() => {
  const list = data.value?.perDeck ?? []
  switch (deckSort.value) {
    case 'winRate':
      return [...list].sort((a, b) => {
        const diff = winRateOf(b.stats) - winRateOf(a.stats)
        return diff !== 0 ? diff : (b.stats?.games_played ?? 0) - (a.stats?.games_played ?? 0)
      })
    case 'gamesPlayed':
      return [...list].sort((a, b) => (b.stats?.games_played ?? 0) - (a.stats?.games_played ?? 0))
    default:
      return list
  }
})

// ------------------------------------------------------------------------- finished games (tab)

const finishedGames = ref<FinishedGame[]>([])
const finishedGamesCursor = ref<string | null>(null)
const finishedGamesLoading = ref(false)
const finishedGamesError = ref(false)
const finishedGamesLoaded = ref(false)

async function loadFinishedGames() {
  finishedGamesLoading.value = true
  finishedGamesError.value = false
  try {
    const page = await listFinishedGames()
    finishedGames.value = page.items
    finishedGamesCursor.value = page.next_cursor
  } catch {
    finishedGamesError.value = true
  } finally {
    finishedGamesLoading.value = false
    finishedGamesLoaded.value = true
  }
}

async function loadMoreFinishedGames() {
  if (!finishedGamesCursor.value) return
  finishedGamesLoading.value = true
  try {
    const page = await listFinishedGames(finishedGamesCursor.value)
    finishedGames.value = [...finishedGames.value, ...page.items]
    finishedGamesCursor.value = page.next_cursor
  } finally {
    finishedGamesLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'games' && !finishedGamesLoaded.value) loadFinishedGames()
})

function winnerOf(game: FinishedGame): FinishedGamePlayer | null {
  return game.players.find((p) => p.won) ?? null
}

/** "1h 12m" / "34m" / "<1m" between started_at and finished_at, or '—' if either is missing. */
function formatDuration(game: FinishedGame): string {
  if (!game.started_at || !game.finished_at) return '—'
  const ms = new Date(game.finished_at).getTime() - new Date(game.started_at).getTime()
  if (ms < 0) return '—'
  const totalMinutes = Math.floor(ms / 60_000)
  if (totalMinutes < 1) return '<1m'
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`
}
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
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('statistics.headToHead.heading') }}</h2>
        <p v-if="!mostPlayedOpponent && !archenemy" class="text-sm" style="color: var(--text-muted);">
          {{ $t('statistics.headToHead.empty') }}
        </p>
        <div v-else class="flex flex-wrap gap-3.5">
          <div
            v-if="mostPlayedOpponent"
            class="min-w-[220px] flex-1 rounded-[24px] border p-5"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('statistics.headToHead.mostPlayed') }}</p>
            <p class="mt-1.5 text-[15px] font-semibold">{{ mostPlayedOpponent.username }}</p>
            <p class="mt-1 text-[13px]" style="color: var(--text-muted);">
              {{ $t('statistics.headToHead.gamesTogether', { count: mostPlayedOpponent.games_together }) }}
            </p>
          </div>
          <div
            v-if="archenemy"
            class="min-w-[220px] flex-1 rounded-[24px] border p-5"
            style="border-color: var(--card-border); background: var(--card-bg);"
          >
            <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('statistics.headToHead.archenemy') }}</p>
            <p class="mt-1.5 text-[15px] font-semibold">{{ archenemy.username }}</p>
            <p class="mt-1 text-[13px]" style="color: var(--lose);">
              {{ $t('statistics.headToHead.eliminatedYou', { count: archenemy.times_eliminated_by_opponent }) }}
            </p>
          </div>
        </div>
      </section>

      <section>
        <h2 class="mb-3.5 text-[15px] font-medium">{{ $t('statistics.mostPlayedGroup.heading') }}</h2>
        <p v-if="!mostPlayedGroup" class="text-sm" style="color: var(--text-muted);">
          {{ $t('statistics.mostPlayedGroup.empty') }}
        </p>
        <NuxtLink
          v-else
          :to="`/playgroups/${mostPlayedGroup.playgroup_id}`"
          class="flex items-center justify-between rounded-[20px] border px-5 py-3.5 transition-colors hover:bg-white/[0.03]"
          style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
        >
          <span class="text-sm font-medium">{{ mostPlayedGroup.playgroup_name }}</span>
          <span class="text-[13px]" style="color: var(--text-dim);">
            {{ $t('statistics.mostPlayedGroup.summary', { count: mostPlayedGroup.games_played }) }}
          </span>
        </NuxtLink>
      </section>

      <section>
        <div role="tablist" class="mb-3.5 flex gap-2.5" @keydown="handleTabKeydown">
          <button
            id="statistics-tab-decks"
            type="button"
            role="tab"
            :aria-selected="activeTab === 'decks'"
            :tabindex="activeTab === 'decks' ? 0 : -1"
            aria-controls="statistics-panel-decks"
            class="rounded-full px-5 py-2.5 text-[13px] font-medium transition-colors"
            :style="activeTab === 'decks'
              ? 'background: linear-gradient(90deg, #8b5cf6, #a855f7); color: #0a0714;'
              : 'border: 1px solid var(--card-border); color: var(--text-muted);'"
            @click="activeTab = 'decks'"
          >
            {{ $t('statistics.tabs.byDeck') }}
          </button>
          <button
            id="statistics-tab-games"
            type="button"
            role="tab"
            :aria-selected="activeTab === 'games'"
            :tabindex="activeTab === 'games' ? 0 : -1"
            aria-controls="statistics-panel-games"
            class="rounded-full px-5 py-2.5 text-[13px] font-medium transition-colors"
            :style="activeTab === 'games'
              ? 'background: linear-gradient(90deg, #8b5cf6, #a855f7); color: #0a0714;'
              : 'border: 1px solid var(--card-border); color: var(--text-muted);'"
            @click="activeTab = 'games'"
          >
            {{ $t('statistics.tabs.finishedGames') }}
          </button>
        </div>

        <div v-if="activeTab === 'decks'" id="statistics-panel-decks" role="tabpanel" aria-labelledby="statistics-tab-decks" tabindex="0">
          <p v-if="!data.perDeck.length" class="text-sm" style="color: var(--text-muted);">
            {{ $t('statistics.perDeck.noDecksIntro') }}
            <NuxtLink to="/decks" style="color: var(--accent-link);">{{ $t('statistics.perDeck.importLink') }}</NuxtLink>
            {{ $t('statistics.perDeck.noDecksOutro') }}
          </p>

          <template v-else>
            <SortSelect
              :model-value="deckSort"
              :options="deckSortOptions"
              :select-label="$t('statistics.perDeck.sort.ariaLabel')"
              class="mb-3.5"
              @update:model-value="(v) => (deckSort = v as DeckSortOrder)"
            />

            <div class="flex flex-col gap-3.5">
              <article
                v-for="entry in sortedDecks"
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
          </template>
        </div>

        <div v-else id="statistics-panel-games" role="tabpanel" aria-labelledby="statistics-tab-games" tabindex="0">
          <p v-if="finishedGamesLoading && !finishedGames.length" class="text-sm" style="color: var(--text-muted);">
            {{ $t('statistics.finishedGames.loading') }}
          </p>
          <p v-else-if="finishedGamesError" class="text-sm" style="color: var(--lose);">{{ $t('statistics.loadError') }}</p>
          <p v-else-if="!finishedGames.length" class="text-sm" style="color: var(--text-muted);">
            {{ $t('statistics.finishedGames.empty') }}
          </p>
          <template v-else>
            <div class="grid grid-cols-1 gap-3.5 lg:grid-cols-2">
              <article
                v-for="game in finishedGames"
                :key="game.id"
                class="rounded-[28px] border p-[22px]"
                style="border-color: var(--card-border); background: var(--card-bg);"
              >
                <div class="flex flex-wrap items-baseline justify-between gap-2">
                  <h3 class="text-[15px] font-medium">{{ game.playgroup_name ?? $t('statistics.finishedGames.noGroup') }}</h3>
                  <span class="text-[13px]" style="color: var(--text-dim);">
                    {{ game.finished_at ? d(new Date(game.finished_at), 'short') : '' }}
                  </span>
                </div>
                <div class="mt-3.5 flex flex-col gap-4 sm:flex-row">
                  <div class="grid w-[164px] flex-shrink-0 grid-cols-2 gap-2 sm:w-[196px]">
                    <div
                      v-for="player in game.players"
                      :key="player.user_id"
                      class="relative flex flex-col items-center gap-1.5 rounded-2xl px-1.5 py-2 text-center"
                      :style="player.won ? 'background: rgba(52,211,153,0.10); box-shadow: 0 0 0 1.5px var(--win) inset;' : ''"
                    >
                      <DeckArt
                        :deck="{ id: player.deck_id, user_id: '', name: player.deck_name, commander: player.deck_commander, moxfield_id: null, image_url: player.deck_image_url }"
                        class="h-12 w-12 sm:h-14 sm:w-14"
                        rounded="rounded-xl"
                      />
                      <span
                        v-if="player.won"
                        :title="$t('statistics.finishedGames.winner')"
                        :aria-label="$t('statistics.finishedGames.winner')"
                        class="absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full text-[11px]"
                        style="background: var(--win); color: #05261c;"
                      >
                        <span aria-hidden="true">🏆</span>
                      </span>
                      <p
                        class="w-full truncate text-[11px]"
                        :style="player.won ? 'color: var(--win); font-weight: 600;' : 'color: var(--text-muted);'"
                      >
                        {{ player.username }}
                      </p>
                    </div>
                  </div>

                  <div class="grid flex-1 grid-cols-2 content-start gap-x-3 gap-y-3">
                    <div>
                      <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.finishedGames.winner') }}</p>
                      <p class="mt-1 truncate text-sm font-semibold" style="color: var(--win);">{{ winnerOf(game)?.username ?? '—' }}</p>
                    </div>
                    <div>
                      <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.finishedGames.duration') }}</p>
                      <p class="mt-1 text-sm font-semibold">{{ formatDuration(game) }}</p>
                    </div>
                    <div>
                      <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.finishedGames.turns') }}</p>
                      <p class="mt-1 text-sm font-semibold">{{ game.turn_count || '—' }}</p>
                    </div>
                    <div>
                      <p class="text-[11px] uppercase" style="color: var(--text-dim);">{{ $t('statistics.finishedGames.biggestHit') }}</p>
                      <p class="mt-1 truncate text-sm font-semibold">
                        <template v-if="game.biggest_hit">
                          {{ game.biggest_hit.amount }}
                          <span class="font-normal" style="color: var(--text-dim);">({{ game.biggest_hit.username }})</span>
                        </template>
                        <template v-else>—</template>
                      </p>
                    </div>
                  </div>
                </div>
              </article>
            </div>
            <div v-if="finishedGamesCursor" class="mt-3.5 flex justify-center">
              <button
                type="button"
                :disabled="finishedGamesLoading"
                class="rounded-full border px-5 py-2.5 text-[13px] disabled:opacity-50"
                style="border-color: var(--input-border); color: var(--text);"
                @click="loadMoreFinishedGames"
              >
                {{ finishedGamesLoading ? $t('statistics.finishedGames.loadingMore') : $t('statistics.finishedGames.loadMore') }}
              </button>
            </div>
          </template>
        </div>
      </section>
    </template>
  </div>
</template>
