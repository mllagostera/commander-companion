<script setup lang="ts">
import type { AdminDailyActivityPoint } from '~/types/api'

const props = defineProps<{ points: AdminDailyActivityPoint[] }>()

const { t, locale } = useI18n()

// viewBox dimensions. The wrapper div locks its aspect-ratio to VB_W/VB_H (see
// template), so a fraction of the rendered width always equals the same
// fraction of the viewBox — no SVG-to-screen matrix math needed to place the
// hover crosshair or the tooltip.
const VB_W = 760
const VB_H = 220
const PAD_LEFT = 34
const PAD_RIGHT = 12
const PAD_TOP = 14
const PAD_BOTTOM = 26
const plotWidth = VB_W - PAD_LEFT - PAD_RIGHT
const plotHeight = VB_H - PAD_TOP - PAD_BOTTOM

const yMax = computed(() => {
  const values = props.points.flatMap((p) => [p.games_started, p.active_users])
  const rawMax = Math.max(1, ...values)
  // 20% headroom so the highest point never touches the top gridline.
  return Math.ceil(rawMax * 1.2)
})

function xAt(index: number): number {
  const n = props.points.length
  if (n <= 1) return PAD_LEFT + plotWidth / 2
  return PAD_LEFT + (index / (n - 1)) * plotWidth
}

function yAt(value: number): number {
  return PAD_TOP + plotHeight * (1 - value / yMax.value)
}

function linePath(series: 'games_started' | 'active_users'): string {
  return props.points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${xAt(i)} ${yAt(p[series])}`).join(' ')
}

const gamesPath = computed(() => linePath('games_started'))
const usersPath = computed(() => linePath('active_users'))

// 3 recessive gridlines: 0, half, max.
const gridLines = computed(() => [0, yMax.value / 2, yMax.value])

const dateFormatter = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric' }))
function formatDate(iso: string): string {
  return dateFormatter.value.format(new Date(`${iso}T00:00:00Z`))
}

// A handful of x-axis labels, not one per point: first, last, and evenly
// spaced ones in between, capped so labels never crowd on a wide range.
const xLabelIndices = computed(() => {
  const n = props.points.length
  if (n === 0) return []
  if (n <= 6) return props.points.map((_, i) => i)
  const maxLabels = 6
  const step = Math.ceil((n - 1) / (maxLabels - 1))
  const indices = []
  for (let i = 0; i < n - 1; i += step) indices.push(i)
  indices.push(n - 1)
  return indices
})

const hoverIndex = ref<number | null>(null)
const wrapperRef = ref<HTMLElement | null>(null)

function onMouseMove(event: MouseEvent) {
  const el = wrapperRef.value
  if (!el || props.points.length === 0) return
  const rect = el.getBoundingClientRect()
  const fraction = (event.clientX - rect.left) / rect.width
  const svgX = fraction * VB_W
  const n = props.points.length
  if (n <= 1) {
    hoverIndex.value = 0
    return
  }
  const rawIndex = Math.round(((svgX - PAD_LEFT) / plotWidth) * (n - 1))
  hoverIndex.value = Math.min(n - 1, Math.max(0, rawIndex))
}

function onMouseLeave() {
  hoverIndex.value = null
}

const hoverPoint = computed(() => (hoverIndex.value === null ? null : props.points[hoverIndex.value]))
const tooltipLeftPct = computed(() => (hoverIndex.value === null ? 0 : (xAt(hoverIndex.value) / VB_W) * 100))
// Flip the tooltip to the left half once past the chart's midpoint, so it
// never runs off the right edge of the card.
const tooltipAlign = computed(() => (tooltipLeftPct.value > 60 ? 'right' : 'left'))
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-wrap items-center gap-4 text-[13px]" style="color: var(--text-muted);">
      <span class="flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-full" style="background: var(--chart-series-1);" />
        {{ t('admin.overview.chart.gamesStarted') }}
      </span>
      <span class="flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-full" style="background: var(--chart-series-2);" />
        {{ t('admin.overview.chart.activeUsers') }}
      </span>
    </div>

    <div
      v-if="points.length"
      ref="wrapperRef"
      class="relative w-full"
      :style="{ aspectRatio: `${VB_W} / ${VB_H}` }"
      @mousemove="onMouseMove"
      @mouseleave="onMouseLeave"
    >
      <svg :viewBox="`0 0 ${VB_W} ${VB_H}`" class="h-full w-full" preserveAspectRatio="none" role="presentation">
        <!-- Recessive gridlines + value labels -->
        <g v-for="v in gridLines" :key="v">
          <line
            :x1="PAD_LEFT" :x2="VB_W - PAD_RIGHT" :y1="yAt(v)" :y2="yAt(v)"
            stroke="var(--card-border)" stroke-width="1"
          />
          <text
:x="PAD_LEFT - 8" :y="yAt(v)" text-anchor="end" dominant-baseline="middle"
            fill="var(--text-dim)" font-size="10"
          >{{ Math.round(v) }}</text>
        </g>

        <!-- X-axis date labels -->
        <text
          v-for="i in xLabelIndices" :key="`x-${i}`"
          :x="xAt(i)" :y="VB_H - PAD_BOTTOM + 16" text-anchor="middle"
          fill="var(--text-dim)" font-size="10"
        >{{ formatDate(points[i]!.date) }}</text>

        <!-- Data lines: thin, rounded joins, no fill -->
        <path
:d="gamesPath" fill="none" stroke="var(--chart-series-1)" stroke-width="2"
          stroke-linecap="round" stroke-linejoin="round"
        />
        <path
:d="usersPath" fill="none" stroke="var(--chart-series-2)" stroke-width="2"
          stroke-linecap="round" stroke-linejoin="round"
        />

        <!-- Hover crosshair + point markers -->
        <template v-if="hoverPoint && hoverIndex !== null">
          <line
            :x1="xAt(hoverIndex)" :x2="xAt(hoverIndex)" :y1="PAD_TOP" :y2="VB_H - PAD_BOTTOM"
            stroke="var(--text-dim)" stroke-width="1" stroke-dasharray="2,3"
          />
          <circle
:cx="xAt(hoverIndex)" :cy="yAt(hoverPoint.games_started)" r="3.5"
            fill="var(--page-solid)" stroke="var(--chart-series-1)" stroke-width="2"
          />
          <circle
:cx="xAt(hoverIndex)" :cy="yAt(hoverPoint.active_users)" r="3.5"
            fill="var(--page-solid)" stroke="var(--chart-series-2)" stroke-width="2"
          />
        </template>
      </svg>

      <div
        v-if="hoverPoint"
        class="pointer-events-none absolute top-0 z-10 flex flex-col gap-1 rounded-[var(--radius-sm)] border px-3 py-2 text-[12px] shadow-[0_8px_20px_rgba(0,0,0,0.25)]"
        :style="{
          background: 'var(--menu-bg)',
          borderColor: 'var(--card-border)',
          left: tooltipAlign === 'left' ? `${tooltipLeftPct}%` : undefined,
          right: tooltipAlign === 'right' ? `${100 - tooltipLeftPct}%` : undefined,
          transform: tooltipAlign === 'left' ? 'translateX(12px)' : 'translateX(-12px)',
        }"
      >
        <span class="font-semibold" style="color: var(--text);">{{ formatDate(hoverPoint.date) }}</span>
        <span class="flex items-center gap-1.5" style="color: var(--text-muted);">
          <span class="h-2 w-2 rounded-full" style="background: var(--chart-series-1);" />
          {{ t('admin.overview.chart.gamesStarted') }}: {{ hoverPoint.games_started }}
        </span>
        <span class="flex items-center gap-1.5" style="color: var(--text-muted);">
          <span class="h-2 w-2 rounded-full" style="background: var(--chart-series-2);" />
          {{ t('admin.overview.chart.activeUsers') }}: {{ hoverPoint.active_users }}
        </span>
      </div>
    </div>

    <p v-else class="text-sm" style="color: var(--text-muted);">{{ t('admin.overview.chart.empty') }}</p>

    <!-- Accessible data table, visually hidden: same data as the chart, for screen
         readers and anyone who can't resolve the SVG's color-coded lines. -->
    <table class="sr-only">
      <caption>{{ t('admin.overview.chart.tableCaption') }}</caption>
      <thead>
        <tr>
          <th scope="col">{{ t('admin.overview.chart.date') }}</th>
          <th scope="col">{{ t('admin.overview.chart.gamesStarted') }}</th>
          <th scope="col">{{ t('admin.overview.chart.activeUsers') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in points" :key="p.date">
          <td>{{ p.date }}</td>
          <td>{{ p.games_started }}</td>
          <td>{{ p.active_users }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
