<script setup lang="ts">
definePageMeta({ middleware: 'admin' })

const { getOverviewStats } = useAdmin()

const { data: stats, error: statsError } = await useAsyncData(
  'admin-overview-stats',
  () => getOverviewStats(),
  { default: () => null },
)

const tiles = computed(() => {
  if (!stats.value) return []
  const s = stats.value
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

    <p v-if="statsError" class="text-sm" style="color: var(--lose);">{{ $t('admin.overview.loadError') }}</p>

    <div v-else class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
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
  </div>
</template>
