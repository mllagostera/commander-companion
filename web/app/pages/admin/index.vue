<script setup lang="ts">
definePageMeta({ middleware: 'admin' })

const { getOverviewStats, getDailyActivity } = useAdmin()

const { data, error: loadError } = await useAsyncData(
  'admin-overview',
  async () => {
    const [stats, activity] = await Promise.all([getOverviewStats(), getDailyActivity()])
    return { stats, activity }
  },
  { default: () => null },
)

const tiles = computed(() => {
  if (!data.value) return []
  const s = data.value.stats
  return [
    { key: 'totalUsers', value: s.total_users },
    { key: 'activeUsers', value: s.active_users },
    { key: 'verifiedUsers', value: s.verified_users },
    { key: 'totalDecks', value: s.total_decks },
    { key: 'totalPlaygroups', value: s.total_playgroups },
    { key: 'totalFinishedGames', value: s.total_finished_games },
    { key: 'totalTournaments', value: s.total_tournaments },
  ]
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <section>
      <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('admin.overview.title') }}</h1>
      <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('admin.overview.subtitle') }}</p>
    </section>

    <NuxtLink
      to="/admin/users"
      class="inline-flex w-fit items-center gap-2 rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.04]"
      style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
    >
      {{ $t('admin.overview.manageUsers') }}
    </NuxtLink>

    <p v-if="loadError" class="text-sm" style="color: var(--lose);">{{ $t('admin.overview.loadError') }}</p>

    <template v-else-if="data">
      <!-- Live activity: online users + active games. Amber (--warn) marks it as a
           right-now, in-progress signal, same token already used elsewhere for a
           running game/tournament — distinct from the neutral totals below. -->
      <section
        class="flex flex-wrap items-center gap-8 rounded-[var(--radius-xl)] border p-5"
        style="border-color: rgba(251,191,36,0.3); background: var(--warn-bg);"
      >
        <div class="flex items-center gap-2.5">
          <span class="relative flex h-2.5 w-2.5">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60" style="background: var(--warn);" />
            <span class="relative inline-flex h-2.5 w-2.5 rounded-full" style="background: var(--warn);" />
          </span>
          <span class="text-[11px] font-semibold uppercase tracking-wide" style="color: var(--warn);">
            {{ $t('admin.overview.live.label') }}
          </span>
        </div>
        <div class="flex flex-col">
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
            {{ $t('admin.overview.live.onlineUsers') }}
          </span>
          <span class="text-2xl font-semibold" style="color: var(--text);">{{ data.stats.online_users }}</span>
        </div>
        <div class="flex flex-col">
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
            {{ $t('admin.overview.live.activeGames') }}
          </span>
          <span class="text-2xl font-semibold" style="color: var(--text);">{{ data.stats.active_games }}</span>
        </div>
      </section>

      <section class="rounded-[var(--radius-xl)] border p-5" style="border-color: var(--card-border); background: var(--card-bg);">
        <h2 class="text-[15px] font-medium" style="color: var(--text);">{{ $t('admin.overview.chart.title') }}</h2>
        <p class="mt-1 text-[13px]" style="color: var(--text-muted);">{{ $t('admin.overview.chart.subtitle') }}</p>
        <div class="mt-4">
          <AdminActivityChart :points="data.activity" />
        </div>
      </section>

      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        <div
          v-for="tile in tiles"
          :key="tile.key"
          class="flex flex-col gap-1.5 rounded-[var(--radius-xl)] border p-5"
          style="border-color: var(--card-border); background: var(--card-bg);"
        >
          <span class="text-[11px] uppercase tracking-wide" style="color: var(--text-dim);">
            {{ $t(`admin.overview.stats.${tile.key}`) }}
          </span>
          <span class="text-2xl font-semibold" style="color: var(--text);">{{ tile.value }}</span>
        </div>
      </div>
    </template>
  </div>
</template>
