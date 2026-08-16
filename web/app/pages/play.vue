<script setup lang="ts">
import type { LocalPlayer } from '~/composables/useLocalGame'

definePageMeta({ layout: false })

const { t } = useI18n()

const {
  players, turn, isFinished, winnerId, started, startingPlayerId, currentTurnPlayerId, paused,
  lotteryActive, lotteryHighlightId, showStarterBanner,
  setup, startRandomPlayer, adjustLife, adjustPoison, adjustCommanderDamage, passTurn, togglePause, resetLives,
  finishManually, reset,
  isEliminated,
} = useLocalGame()

const { isFullscreen, isSupported: isFullscreenSupported, toggleFullscreen } = useFullscreen()

type Phase = 'setup' | 'tracker'
const phase = ref<Phase>('setup')

// ------------------------------------------------------------------- setup
const playerCount = ref(4)
const playerNames = ref<string[]>(Array.from({ length: 6 }, () => ''))

function startGame() {
  const names = playerNames.value.slice(0, playerCount.value)
  setup(names)
  phase.value = 'tracker'
}

function backToSetup() {
  reset()
  phase.value = 'setup'
}

// ----------------------------------------------------------------- tracker
const expandedPlayerId = ref<number | null>(null)

function toggleExpand(playerId: number) {
  expandedPlayerId.value = expandedPlayerId.value === playerId ? null : playerId
}

const winner = computed<LocalPlayer | null>(() => players.value.find((p) => p.id === winnerId.value) ?? null)

const topRow = computed(() => players.value.slice(0, Math.ceil(players.value.length / 2)))
const bottomRow = computed(() => players.value.slice(Math.ceil(players.value.length / 2)))

/** Spells "Turno N" as a ring of characters orbiting the pause button (`cc-orbit-spin`
 * in main.css spins the whole ring). */
const orbitChars = computed(() => {
  const label = t('play.tracker.turn', { n: turn.value })
  const chars = label.split('')
  const arc = Math.min(160, chars.length * 18)
  const start = -arc / 2
  const step = chars.length > 1 ? arc / (chars.length - 1) : 0
  return chars.map((ch, i) => ({ ch, angle: start + i * step }))
})

function dealtBy(player: LocalPlayer): number {
  return players.value.reduce((sum, p) => sum + (p.commanderDamage[player.id] ?? 0), 0)
}

function takenBy(player: LocalPlayer): number {
  const values = Object.values(player.commanderDamage)
  return values.length ? Math.max(...values) : 0
}

function statusKey(player: LocalPlayer): string {
  if (winner.value?.id === player.id) return 'play.summary.statusWinner'
  if (isEliminated(player)) return 'play.summary.statusEliminated'
  return 'play.summary.statusInPlay'
}

const starterName = computed(() => players.value.find((p) => p.id === startingPlayerId.value)?.name ?? '')
const starterColor = computed(() => players.value.find((p) => p.id === startingPlayerId.value)?.color ?? '#8b5cf6')

// The tracker only makes sense in landscape (same as Android, which forces landscape via
// RotateDevicePrompt) — detected with matchMedia instead of a simulated timer.
//
// WCAG 1.3.4 (Orientation) exception: portrait is intentionally blocked rather than offered
// as a degraded alternative layout. A life-total grid for 2-6 players needs the width; a
// portrait fallback would mean maintaining a second, cramped tracker layout for a game screen
// people already play holding their device landscape. This is the documented "essential"
// exception the criterion allows, not an oversight.
const isPortrait = ref(false)
let orientationQuery: MediaQueryList | null = null
function updateOrientation() {
  isPortrait.value = orientationQuery?.matches ?? false
}
onMounted(() => {
  orientationQuery = window.matchMedia('(orientation: portrait)')
  updateOrientation()
  orientationQuery.addEventListener('change', updateOrientation)
})
onUnmounted(() => {
  orientationQuery?.removeEventListener('change', updateOrientation)
})
</script>

<template>
  <div>
    <main
      v-if="phase === 'setup'"
      class="relative min-h-screen overflow-hidden"
      style="background: var(--bg-gradient); color: var(--text);"
    >
      <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
      <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />

      <div class="relative z-[1] flex min-h-screen flex-col items-center justify-center gap-6 p-6">
        <NuxtLink to="/" class="flex items-center gap-2.5">
          <AppLogo />
          <span class="cc-gradient-text text-[15px] font-semibold tracking-wide">Commander Companion</span>
        </NuxtLink>

        <section class="flex w-full max-w-md flex-col gap-6 rounded-[28px] border p-[26px]" style="background: var(--card-bg-strong); border-color: var(--card-border);">
          <div>
            <h1 class="text-2xl font-semibold sm:text-[26px]">{{ $t('play.setup.title') }}</h1>
            <p class="mt-2 text-sm" style="color: var(--text-muted);">{{ $t('play.setup.subtitle') }}</p>
          </div>

          <div>
            <p class="text-xs" style="color: var(--text-dim);">{{ $t('play.setup.playerCountLabel') }}</p>
            <div class="mt-2 flex gap-2">
              <button
                v-for="n in [2, 3, 4, 5, 6]"
                :key="n"
                type="button"
                class="h-10 w-10 rounded-full border text-sm font-semibold"
                :style="n === playerCount
                  ? { background: 'linear-gradient(90deg, #8b5cf6, #a855f7)', color: '#0a0714', borderColor: 'transparent' }
                  : { background: 'var(--input-bg)', borderColor: 'var(--input-border)', color: 'var(--text)' }"
                @click="playerCount = n"
              >
                {{ n }}
              </button>
            </div>
          </div>

          <div class="flex flex-col gap-2.5">
            <div v-for="i in playerCount" :key="i" class="flex items-center gap-2.5">
              <span
                class="h-8 w-8 flex-shrink-0 rounded-full"
                :style="{ background: LOCAL_PLAYER_COLORS[(i - 1) % LOCAL_PLAYER_COLORS.length] }"
              />
              <input
                v-model="playerNames[i - 1]"
                type="text"
                :placeholder="$t('play.setup.playerPlaceholder', { n: i })"
                :aria-label="$t('play.setup.playerPlaceholder', { n: i })"
                class="flex-1 rounded-full border px-4 py-2.5 text-[13px] outline-none"
                style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
              >
            </div>
          </div>

          <button
            type="button"
            class="rounded-full px-5 py-3 text-sm font-semibold text-[#0a0714] shadow-[0_6px_20px_rgba(139,92,246,0.35)] transition-transform hover:scale-[1.02]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            @click="startGame"
          >
            {{ $t('play.setup.start') }}
          </button>

          <p class="text-center text-xs" style="color: var(--text-dim);">{{ $t('play.setup.localNote') }}</p>
        </section>

        <button type="button" class="border-none bg-transparent px-2 py-1 text-[13px]" style="color: var(--text-dim);" @click="navigateTo('/')">
          {{ $t('play.setup.cancel') }}
        </button>
      </div>
    </main>

    <ClientOnly>
      <div v-if="phase === 'tracker'" class="fixed inset-0 overflow-hidden" style="background: #0a0714;">
        <div v-if="isPortrait" class="flex h-full w-full flex-col items-center justify-center gap-3 px-8 text-center">
          <span aria-hidden="true" class="text-4xl">📱</span>
          <p class="whitespace-pre-line text-sm font-medium" style="color: #f1f0f6;">{{ $t('play.tracker.rotatePrompt') }}</p>
        </div>

        <template v-else>
          <div v-if="isFinished" class="flex h-full w-full flex-col items-center gap-4 overflow-y-auto px-8 py-6">
            <p class="text-[11px] tracking-wide" style="color: #8b87a3;">{{ $t('play.summary.heading') }}</p>

            <div
              v-if="winner"
              class="rounded-[18px_12px_18px_12px] px-5 py-2"
              :style="{ background: winner.color }"
            >
              <p class="text-sm font-bold" style="color: #000;">{{ $t('play.summary.winner', { name: winner.name }) }}</p>
            </div>
            <p v-else class="text-sm font-semibold" style="color: #f1f0f6;">{{ $t('play.summary.draw') }}</p>

            <div class="flex w-full max-w-2xl flex-1 flex-col gap-1.5 overflow-y-auto">
              <div class="flex gap-1 px-2.5 text-[9px]" style="color: #8b87a3;">
                <span class="flex-[1.3]">{{ $t('play.summary.columnPlayer') }}</span>
                <span class="flex-[0.7]">{{ $t('play.summary.columnLife') }}</span>
                <span class="flex-[0.9]">{{ $t('play.summary.columnDealt') }}</span>
                <span class="flex-[0.9]">{{ $t('play.summary.columnTaken') }}</span>
                <span class="flex-[0.7]">{{ $t('play.summary.columnPoison') }}</span>
                <span class="flex-[0.9]">{{ $t('play.summary.columnStatus') }}</span>
              </div>
              <div
                v-for="player in players"
                :key="player.id"
                class="flex items-center gap-1 rounded-xl border px-2.5 py-2"
                style="border-color: rgba(255,255,255,0.08); background: rgba(255,255,255,0.03);"
              >
                <span class="flex flex-[1.3] items-center gap-1.5 text-[11px]" style="color: #f1f0f6;">
                  <span class="h-2 w-2 rounded-full" :style="{ background: player.color }" />
                  {{ player.name }}
                </span>
                <span class="flex-[0.7] text-[11px]" style="color: #a5a3b8;">{{ player.life }}</span>
                <span class="flex-[0.9] text-[11px]" style="color: #a5a3b8;">{{ dealtBy(player) }}</span>
                <span class="flex-[0.9] text-[11px]" style="color: #a5a3b8;">{{ takenBy(player) }}</span>
                <span class="flex-[0.7] text-[11px]" style="color: #a5a3b8;">{{ player.poison }}</span>
                <span class="flex-[0.9] text-[10px] font-semibold" style="color: #c4b5fd;">{{ $t(statusKey(player)) }}</span>
              </div>
            </div>

            <div class="flex gap-3 pb-2">
              <button
                type="button"
                class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714]"
                style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
                @click="backToSetup"
              >
                {{ $t('play.summary.newGame') }}
              </button>
              <NuxtLink to="/" class="rounded-full border px-5 py-2.5 text-[13px]" style="border-color: rgba(255,255,255,0.15); color: #f1f0f6;">
                {{ $t('play.summary.backHome') }}
              </NuxtLink>
            </div>
          </div>

          <template v-else>
            <div class="flex h-full w-full flex-col gap-1 p-1">
              <div class="flex flex-1 gap-1">
                <PlayQuadrant
                  v-for="player in topRow"
                  :key="player.id"
                  :player="player"
                  :all-players="players"
                  :rotated="true"
                  :started="started"
                  :highlighted="lotteryHighlightId === player.id"
                  :is-current-turn="started && currentTurnPlayerId === player.id"
                  :expanded-id="expandedPlayerId"
                  @toggle-expand="toggleExpand"
                  @adjust-life="adjustLife"
                  @adjust-poison="adjustPoison"
                  @adjust-commander-damage="adjustCommanderDamage"
                  @pass-turn="passTurn"
                />
              </div>

              <div class="flex flex-1 gap-1">
                <PlayQuadrant
                  v-for="player in bottomRow"
                  :key="player.id"
                  :player="player"
                  :all-players="players"
                  :rotated="false"
                  :started="started"
                  :highlighted="lotteryHighlightId === player.id"
                  :is-current-turn="started && currentTurnPlayerId === player.id"
                  :expanded-id="expandedPlayerId"
                  @toggle-expand="toggleExpand"
                  @adjust-life="adjustLife"
                  @adjust-poison="adjustPoison"
                  @adjust-commander-damage="adjustCommanderDamage"
                  @pass-turn="passTurn"
                />
              </div>
            </div>

            <span
              class="pointer-events-none absolute inset-0 flex items-center justify-center text-[96px] font-extrabold"
              style="color: rgba(255,255,255,0.05);"
            >{{ turn }}</span>

            <div v-if="started" class="pointer-events-none absolute left-1/2 top-1/2 z-[2] h-px w-px">
              <div class="absolute left-1/2 top-1/2 h-px w-px animate-[cc-orbit-spin_9s_linear_infinite]">
                <span
                  v-for="(c, i) in orbitChars"
                  :key="i"
                  class="absolute left-0 top-0 flex items-center justify-center text-[9px] font-bold leading-none"
                  style="width: 12px; height: 12px; margin-left: -6px; margin-top: -6px; color: #fff; text-shadow: 0 1px 4px rgba(0,0,0,0.6);"
                  :style="{ transform: `rotate(${c.angle}deg) translateY(-62px)` }"
                >{{ c.ch }}</span>
              </div>
            </div>

            <button
              v-if="!started && !lotteryActive"
              type="button"
              :title="$t('play.tracker.startRandom')"
              :aria-label="$t('play.tracker.startRandom')"
              class="absolute left-1/2 top-1/2 flex h-[72px] w-[72px] -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full shadow-[0_10px_30px_rgba(139,92,246,0.45)]"
              style="background: linear-gradient(135deg, #8b5cf6, #a855f7);"
              @click="startRandomPlayer"
            >
              <span
                class="ml-1.5 inline-block h-0 w-0"
                style="border-top: 11px solid transparent; border-bottom: 11px solid transparent; border-left: 17px solid #0a0714;"
              />
            </button>

            <button
              v-if="started"
              type="button"
              :title="$t('play.tracker.pause')"
              :aria-label="$t('play.tracker.pause')"
              class="absolute left-1/2 top-1/2 z-[3] flex h-14 w-14 -translate-x-1/2 -translate-y-1/2 items-center justify-center gap-1 rounded-full border shadow-[0_8px_24px_rgba(0,0,0,0.5)]"
              style="border-color: rgba(196,181,253,0.4); background: rgba(10,7,20,0.85);"
              @click="togglePause"
            >
              <span class="h-4 w-1 rounded-sm" style="background: #fff;" />
              <span class="h-4 w-1 rounded-sm" style="background: #fff;" />
            </button>

            <div v-if="showStarterBanner" class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3.5" style="background: rgba(5,3,8,0.94);">
              <span class="h-4 w-4 rounded-full" :style="{ background: starterColor, boxShadow: `0 0 20px ${starterColor}` }" />
              <p class="px-6 text-center font-extrabold" style="color: #ffffff; font-size: clamp(22px, 4vw, 34px);">{{ $t('play.tracker.starterBanner', { name: starterName }) }}</p>
            </div>

            <div v-if="paused" class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3.5" style="background: rgba(5,3,8,0.92);">
              <p class="text-base font-semibold" style="color: #f1f0f6;">{{ $t('play.tracker.paused') }}</p>
              <button
                type="button"
                class="rounded-full px-[26px] py-3 text-[13px] font-bold"
                style="background: linear-gradient(90deg, #8b5cf6, #a855f7); color: #0a0714;"
                @click="togglePause"
              >
                {{ $t('play.tracker.resume') }}
              </button>
              <button type="button" class="border-none bg-transparent p-0 text-[13px]" style="color: #f87171;" @click="resetLives">
                {{ $t('play.tracker.resetLives') }}
              </button>
            </div>

            <button
              type="button"
              :aria-label="$t('play.tracker.exit')"
              class="absolute z-[5] flex items-center justify-center rounded-full border"
              style="left: clamp(8px,1.5vw,16px); top: clamp(8px,1.5vw,16px); width: clamp(36px,5vw,48px); height: clamp(36px,5vw,48px); background: rgba(20,16,38,0.96); border-color: rgba(255,255,255,0.15); color: #f1f0f6; font-size: clamp(14px,1.8vw,18px);"
              @click="backToSetup"
            >
              <span aria-hidden="true">✕</span>
            </button>

            <button
              v-if="isFullscreenSupported"
              type="button"
              :title="$t(isFullscreen ? 'play.tracker.exitFullscreen' : 'play.tracker.fullscreen')"
              :aria-label="$t(isFullscreen ? 'play.tracker.exitFullscreen' : 'play.tracker.fullscreen')"
              class="absolute z-[5] flex items-center justify-center rounded-full border"
              style="left: clamp(52px,7.5vw,72px); top: clamp(8px,1.5vw,16px); width: clamp(36px,5vw,48px); height: clamp(36px,5vw,48px); background: rgba(20,16,38,0.96); border-color: rgba(255,255,255,0.15); color: #f1f0f6; font-size: clamp(14px,1.8vw,18px);"
              @click="toggleFullscreen"
            >
              <span aria-hidden="true">⛶</span>
            </button>

            <button
              v-if="started"
              type="button"
              class="absolute z-[5] rounded-full border"
              style="right: clamp(8px,1.5vw,16px); top: clamp(8px,1.5vw,16px); padding: clamp(8px,1.4vw,12px) clamp(14px,2.5vw,22px); background: rgba(20,16,38,0.96); border-color: rgba(248,113,113,0.35); color: #f87171; font-size: clamp(12px,1.6vw,15px);"
              @click="finishManually"
            >
              {{ $t('play.tracker.finish') }}
            </button>
          </template>
        </template>
      </div>
    </ClientOnly>
  </div>
</template>
