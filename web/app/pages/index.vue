<script setup lang="ts">
import type { DashboardGame } from '~/types/api'

const { d } = useI18n()
const { user } = useAuth()
const { dashboard } = useStatistics()

/**
 * One request. This screen used to assemble itself client-side from six
 * endpoints — every deck page, then every playgroup's entire game history —
 * which cost 30 requests and 539 KB on a 400-game account to show four games
 * (measured 2026-09-02, see docs/roadmap/DECISIONS-LOG.md). The server now
 * returns the screen, already sliced and already resolved from this user's
 * seat, in a fixed number of queries.
 */
const { data, error, refresh } = await useAsyncData('dashboard', () => dashboard())

// useAsyncData reuses the cached payload when returning to this page in the same session (e.g.
// after finishing a game on Android): without this refresh, the summary would be stale.
//
// It's skipped during hydration, where the payload was fetched by this very
// render and re-fetching it only doubles the cost of every hard load.
const nuxtApp = useNuxtApp()
onMounted(() => {
  if (nuxtApp.isHydrating) return
  refresh()
})

function gameDate(game: DashboardGame): string {
  const iso = game.finished_at ?? game.started_at
  return iso ? d(new Date(iso), 'short') : '—'
}
</script>

<template>
  <div class="flex flex-col gap-8">
    <section class="flex flex-wrap items-center justify-between gap-4">
      <h1 class="text-[26px] font-semibold sm:text-[32px]">
        {{ $t('dashboard.greeting', { username: user?.username ?? '…' }) }}
      </h1>
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
      <!-- Spotlight: the best deck's art carries the page, with performance beside it. -->
      <section class="grid grid-cols-1 gap-4 lg:grid-cols-[1.35fr_1fr]">
        <NuxtLink
          v-if="data.best_deck"
          to="/statistics"
          class="group relative flex min-h-[230px] flex-col justify-end overflow-hidden rounded-[var(--radius-xl)] border p-6"
          style="border-color: var(--card-border);"
        >
          <DeckArt :deck="data.best_deck" fill image-position="right" class="transition-transform duration-500 group-hover:scale-[1.03]" />
          <div
            class="absolute inset-0"
            style="background: linear-gradient(100deg, rgba(10,7,20,0.95) 25%, rgba(10,7,20,0.72) 60%, rgba(10,7,20,0.35) 100%);"
          />
          <!--
            This card is dark in BOTH themes (the overlay above sits on the art),
            so its text uses fixed light-on-dark values rather than the theme
            tokens: in the light theme --accent-link/--win are dark by design and
            drop to 3.4:1 and 2.7:1 here.
          -->
          <div class="relative">
            <p class="text-[11px] uppercase tracking-wide" style="color: #c4b5fd;">
              {{ $t('dashboard.bestDeck.heading') }}
            </p>
            <p class="mt-2 text-2xl font-semibold leading-tight text-white">{{ data.best_deck.name }}</p>
            <p class="mt-1 text-[13px] text-white/70">{{ data.best_deck.commander }}</p>
            <div class="mt-5 flex items-end gap-6">
              <span>
                <span class="block text-[11px] uppercase tracking-wide text-white/60">{{ $t('dashboard.stats.winRate') }}</span>
                <span class="text-xl font-semibold" style="color: #5eead4;">
                  {{ winRate(data.best_deck.games_played, data.best_deck.games_won) }}
                </span>
              </span>
              <span>
                <span class="block text-[11px] uppercase tracking-wide text-white/60">{{ $t('dashboard.stats.games') }}</span>
                <span class="text-xl font-semibold text-white">{{ data.best_deck.games_played }}</span>
              </span>
            </div>
          </div>
        </NuxtLink>
        <EmptyState
          v-else
          class="min-h-[230px]"
          :title="$t('dashboard.bestDeck.emptyTitle')"
          :body="$t('dashboard.bestDeck.emptyBody')"
          :cta-label="$t('dashboard.bestDeck.emptyCta')"
          cta-to="/decks"
        />

        <div
          class="flex flex-col justify-between gap-5 rounded-[var(--radius-xl)] border p-6"
          style="border-color: var(--card-border); background: var(--card-bg);"
        >
          <div class="flex items-center gap-5">
            <WinRateRing :played="data.stats.games_played" :won="data.stats.games_won" :size="112" />
            <div>
              <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
                {{ $t('dashboard.winLoss.heading') }}
              </p>
              <p class="mt-2 text-[15px]" style="color: var(--text);">
                {{ $t('dashboard.winLoss.won', { count: data.stats.games_won }) }}
              </p>
              <p class="text-[15px]" style="color: var(--text-dim);">
                {{ $t('dashboard.winLoss.lost', { count: data.stats.games_played - data.stats.games_won }) }}
              </p>
            </div>
          </div>

          <div
            class="rounded-[var(--radius-md)] px-4 py-3"
            :style="data.streak
              ? { background: data.streak_won ? 'var(--win-bg)' : 'var(--lose-bg)' }
              : { background: 'var(--dim-bg)' }"
          >
            <!-- --text-muted, not --text-dim like the other section labels: this
                 one sits on the tinted win/lose surface, where dim measures 4.41:1
                 in the dark theme, just under AA. -->
            <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-muted);">
              {{ $t('dashboard.streak.heading') }}
            </p>
            <p
              v-if="data.streak"
              class="mt-0.5 text-sm font-semibold"
              :style="{ color: data.streak_won ? 'var(--win)' : 'var(--lose)' }"
            >
              {{ data.streak_won
                ? $t('dashboard.streak.wins', { count: data.streak })
                : $t('dashboard.streak.losses', { count: data.streak }) }}
            </p>
            <p v-else class="mt-0.5 text-sm" style="color: var(--text-muted);">{{ $t('dashboard.streak.empty') }}</p>
          </div>

          <p class="text-[13px]" style="color: var(--text-dim);">
            {{ $t('dashboard.performance.totals', {
              games: data.stats.games_played,
              decks: data.total_decks,
              groups: data.total_playgroups,
            }) }}
          </p>
        </div>
      </section>

      <!-- Recent games + decks -->
      <section class="grid grid-cols-1 gap-6 lg:grid-cols-[1.35fr_1fr]">
        <div>
          <div class="mb-3.5 flex items-baseline justify-between">
            <h2 class="text-[15px] font-medium">{{ $t('dashboard.recentGames.heading') }}</h2>
            <NuxtLink v-if="data.recent_games.length" to="/statistics" class="-my-1 py-1 text-[13px]" style="color: var(--accent-link);">
              {{ $t('dashboard.recentGames.viewAll') }}
            </NuxtLink>
          </div>

          <EmptyState
            v-if="!data.recent_games.length"
            :title="$t('dashboard.recentGames.emptyTitle')"
            :body="$t('dashboard.recentGames.emptyBody')"
            :cta-label="$t('dashboard.recentGames.emptyCta')"
            cta-to="/play"
          />
          <div v-else class="flex flex-col gap-2.5">
            <div
              v-for="rg in data.recent_games"
              :key="rg.id"
              class="flex items-center gap-3.5 rounded-[var(--radius-md)] border px-4 py-3"
              style="border-color: var(--card-border); background: var(--card-bg);"
            >
              <DeckArt
                v-if="rg.deck"
                :deck="rg.deck"
                class="h-11 w-11 flex-shrink-0"
                rounded="rounded-[var(--radius-sm)]"
              />
              <span
                v-else
                class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-[var(--radius-sm)] text-sm"
                style="background: var(--dim-bg); color: var(--text-dim);"
              >?</span>

              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium">
                  {{ rg.deck?.name ?? $t('dashboard.recentGames.unknownDeck') }}
                </p>
                <p class="truncate text-xs" style="color: var(--text-dim);">
                  {{ rg.opponents.length
                    ? $t('dashboard.recentGames.versus', { opponents: rg.opponents.join(', ') })
                    : rg.playgroup_name }}
                </p>
              </div>

              <div class="flex flex-col items-end gap-1">
                <span
                  class="rounded-full px-2.5 py-0.5 text-[11px] font-semibold"
                  :style="{ background: rg.won ? 'var(--win-bg)' : 'var(--lose-bg)', color: rg.won ? 'var(--win)' : 'var(--lose)' }"
                >
                  {{ rg.won ? $t('dashboard.recentGames.won') : $t('dashboard.recentGames.lost') }}
                </span>
                <!-- The group name is dropped on phones: with it, this column is wide
                     enough to truncate the deck name down to a few characters. -->
                <span class="whitespace-nowrap text-[11px]" style="color: var(--text-dim);">
                  {{ gameDate(rg) }}<span v-if="rg.playgroup_name" class="hidden sm:inline"> · {{ rg.playgroup_name }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>

        <div>
          <div class="mb-3.5 flex items-baseline justify-between">
            <h2 class="text-[15px] font-medium">{{ $t('dashboard.decksSection.heading') }}</h2>
            <NuxtLink v-if="data.decks.length" to="/statistics" class="-my-1 py-1 text-[13px]" style="color: var(--accent-link);">
              {{ $t('dashboard.decksSection.viewStats') }}
            </NuxtLink>
          </div>

          <EmptyState
            v-if="!data.decks.length"
            :title="$t('dashboard.decksSection.emptyTitle')"
            :body="$t('dashboard.decksSection.emptyBody')"
            :cta-label="$t('dashboard.decksSection.emptyCta')"
            cta-to="/decks"
          />
          <div v-else class="flex flex-col gap-2.5">
            <NuxtLink
              v-for="entry in data.decks"
              :key="entry.id"
              to="/decks"
              class="flex items-center gap-3 rounded-[var(--radius-md)] border px-3.5 py-2.5 transition-all hover:translate-x-1"
              style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
            >
              <DeckArt :deck="entry" class="h-12 w-12 flex-shrink-0" rounded="rounded-[var(--radius-sm)]" />
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium">{{ entry.name }}</p>
                <p class="truncate text-xs" style="color: var(--text-dim);">{{ entry.commander }}</p>
              </div>
              <div class="flex-shrink-0 text-right">
                <!-- winRate already renders an em dash at zero games. -->
                <p class="text-sm font-semibold" style="color: var(--accent-link);">
                  {{ winRate(entry.games_played, entry.games_won) }}
                </p>
                <p class="text-[11px]" style="color: var(--text-dim);">
                  {{ $t('dashboard.decksSection.gamesPlayed', entry.games_played) }}
                </p>
              </div>
            </NuxtLink>
          </div>
        </div>
      </section>

      <!-- Groups -->
      <section>
        <div class="mb-3.5 flex items-baseline justify-between">
          <h2 class="text-[15px] font-medium">{{ $t('dashboard.groups.heading') }}</h2>
          <NuxtLink v-if="data.playgroups.length" to="/playgroups" class="-my-1 py-1 text-[13px]" style="color: var(--accent-link);">
            {{ $t('dashboard.groups.viewAll') }}
          </NuxtLink>
        </div>

        <EmptyState
          v-if="!data.playgroups.length"
          :title="$t('dashboard.groups.emptyTitle')"
          :body="$t('dashboard.groups.emptyBody')"
          :cta-label="$t('dashboard.groups.emptyCta')"
          cta-to="/playgroups"
        />
        <div v-else class="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-3">
          <NuxtLink
            v-for="g in data.playgroups"
            :key="g.id"
            :to="`/playgroups/${g.id}`"
            class="flex flex-col gap-3.5 rounded-[var(--radius-lg)] border p-[18px] transition-all hover:-translate-y-1 hover:shadow-[0_14px_34px_rgba(129,140,248,0.18)]"
            style="border-color: var(--card-border); background: var(--card-bg); color: var(--text);"
          >
            <div>
              <p class="text-sm font-medium">{{ g.name }}</p>
              <p class="mt-0.5 text-xs" style="color: var(--text-dim);">
                {{ $t('dashboard.groups.summary', { members: g.member_count, games: g.games_played }) }}
              </p>
            </div>
            <div class="flex">
              <!-- Already capped server-side to what this strip shows. -->
              <span
                v-for="(member, i) in g.members"
                :key="member.user_id"
                class="-ml-2 flex h-7 w-7 items-center justify-center rounded-full border-2 text-[11px] font-semibold text-[#0a0714] first:ml-0"
                :style="{ background: avatarColor(i), borderColor: 'var(--page-solid)' }"
              >
                {{ member.username[0]?.toUpperCase() }}
              </span>
            </div>
          </NuxtLink>
        </div>
      </section>
    </template>
  </div>
</template>
