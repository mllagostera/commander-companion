<script setup lang="ts">
import type { LocalPlayer } from '~/composables/useLocalGame'

definePageMeta({ layout: false })

const {
  players, turn, isFinished, winnerId, started, startingPlayerId,
  setup, beginGame, adjustLife, adjustPoison, adjustCommanderDamage, nextTurn, finishManually, reset,
  isEliminated,
} = useLocalGame()

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
  if (!started.value) return
  expandedPlayerId.value = expandedPlayerId.value === playerId ? null : playerId
}

function opponentsOf(player: LocalPlayer): LocalPlayer[] {
  return players.value.filter((p) => p.id !== player.id)
}

const winner = computed<LocalPlayer | null>(() => players.value.find((p) => p.id === winnerId.value) ?? null)

const topRow = computed(() => players.value.slice(0, Math.ceil(players.value.length / 2)))
const bottomRow = computed(() => players.value.slice(Math.ceil(players.value.length / 2)))

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

// Igual que en Android (GameTrackerScreen): al sortear se muestra un banner ~1.8s con quién
// empieza antes de dejar ver el tablero con los contadores activos.
const showStarterBanner = ref(false)
const starterName = computed(() => players.value.find((p) => p.id === startingPlayerId.value)?.name ?? '')

function handleBeginGame() {
  beginGame()
  showStarterBanner.value = true
  setTimeout(() => { showStarterBanner.value = false }, 1800)
}

// El tracker solo tiene sentido en horizontal (igual que Android, que fuerza landscape con
// RotateDevicePrompt) — se detecta con matchMedia en vez de un timer simulado.
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
      </div>
    </main>

    <ClientOnly>
      <div v-if="phase === 'tracker'" class="fixed inset-0 overflow-hidden" style="background: #0a0714;">
        <div v-if="isPortrait" class="flex h-full w-full flex-col items-center justify-center gap-3 px-8 text-center">
          <span class="text-4xl">📱</span>
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
                <div
                  v-for="player in topRow"
                  :key="player.id"
                  class="relative flex-1 origin-center rotate-180 overflow-hidden rounded-[22px]"
                  :style="{ background: player.color }"
                  @click="toggleExpand(player.id)"
                >
                  <div class="absolute inset-0" style="background: linear-gradient(to bottom, rgba(10,7,20,0.05), rgba(10,7,20,0.45));" />

                  <div v-if="!started" class="relative flex h-full w-full items-center justify-center">
                    <span class="text-[13px] font-semibold" style="color: rgba(0,0,0,0.8);">{{ player.name }}</span>
                  </div>

                  <div v-else class="relative flex h-full w-full flex-col items-center justify-between p-2.5">
                    <p class="text-[11px] font-semibold" style="color: rgba(0,0,0,0.75);">{{ player.name }}</p>

                    <div class="flex items-center gap-3.5">
                      <button type="button" class="flex h-8 w-8 items-center justify-center rounded-full text-lg font-bold" style="background: rgba(0,0,0,0.12); color: #000;" @click.stop="adjustLife(player.id, -1)">−</button>
                      <span class="text-[34px] font-bold tabular-nums" style="color: #000;">{{ player.life }}</span>
                      <button type="button" class="flex h-8 w-8 items-center justify-center rounded-full text-lg font-bold" style="background: rgba(0,0,0,0.12); color: #000;" @click.stop="adjustLife(player.id, 1)">+</button>
                    </div>

                    <div class="flex min-h-[14px] items-center gap-2 text-[10px]" style="color: rgba(0,0,0,0.65);">
                      <span v-if="player.poison > 0">☠ {{ player.poison }}</span>
                      <span v-for="opponent in opponentsOf(player)" :key="opponent.id">
                        <span v-if="(player.commanderDamage[opponent.id] ?? 0) > 0" class="inline-flex items-center gap-1">
                          <span class="h-1.5 w-1.5 rounded-full" :style="{ background: opponent.color }" />
                          {{ player.commanderDamage[opponent.id] }}
                        </span>
                      </span>
                    </div>

                    <button
                      type="button"
                      class="rounded-full border px-3.5 py-1 text-[10px] font-semibold"
                      style="background: rgba(255,255,255,0.2); border-color: rgba(0,0,0,0.3); color: rgba(0,0,0,0.75);"
                      @click.stop="nextTurn"
                    >
                      {{ $t('play.tracker.passTurn') }}
                    </button>
                  </div>

                  <div v-if="expandedPlayerId === player.id" class="absolute inset-0 flex items-center justify-center" style="background: rgba(5,3,8,0.97);" @click.stop="toggleExpand(player.id)">
                    <div class="flex flex-col items-center gap-1.5">
                      <p class="text-[9px] tracking-wide" style="color: #c4b5fd;">{{ $t('play.tracker.commanderDamage') }}</p>
                      <div v-for="opponent in opponentsOf(player)" :key="opponent.id" class="flex items-center gap-1.5">
                        <span class="h-2.5 w-2.5 rounded-full" :style="{ background: opponent.color }" />
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustCommanderDamage(player.id, opponent.id, -1)">−</button>
                        <span class="w-4 text-center text-[11px] tabular-nums" style="color: #fff;">{{ player.commanderDamage[opponent.id] ?? 0 }}</span>
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustCommanderDamage(player.id, opponent.id, 1)">+</button>
                      </div>
                      <div class="mt-1 flex items-center gap-1.5">
                        <span class="text-[9px]" style="color: #a78bfa;">{{ $t('play.tracker.poison') }}</span>
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustPoison(player.id, -1)">−</button>
                        <span class="w-4 text-center text-[11px] tabular-nums" style="color: #fff;">{{ player.poison }}</span>
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustPoison(player.id, 1)">+</button>
                      </div>
                    </div>
                  </div>

                  <div v-if="started && isEliminated(player)" class="absolute inset-0 flex items-center justify-center" style="background: rgba(0,0,0,0.9);">
                    <span class="text-[19px] font-light tracking-[2px] text-white">{{ $t('play.tracker.dead') }}</span>
                  </div>
                </div>
              </div>

              <div class="flex flex-1 gap-1">
                <div
                  v-for="player in bottomRow"
                  :key="player.id"
                  class="relative flex-1 overflow-hidden rounded-[22px]"
                  :style="{ background: player.color }"
                  @click="toggleExpand(player.id)"
                >
                  <div class="absolute inset-0" style="background: linear-gradient(to bottom, rgba(10,7,20,0.05), rgba(10,7,20,0.45));" />

                  <div v-if="!started" class="relative flex h-full w-full items-center justify-center">
                    <span class="text-[13px] font-semibold" style="color: rgba(0,0,0,0.8);">{{ player.name }}</span>
                  </div>

                  <div v-else class="relative flex h-full w-full flex-col items-center justify-between p-2.5">
                    <p class="text-[11px] font-semibold" style="color: rgba(0,0,0,0.75);">{{ player.name }}</p>

                    <div class="flex items-center gap-3.5">
                      <button type="button" class="flex h-8 w-8 items-center justify-center rounded-full text-lg font-bold" style="background: rgba(0,0,0,0.12); color: #000;" @click.stop="adjustLife(player.id, -1)">−</button>
                      <span class="text-[34px] font-bold tabular-nums" style="color: #000;">{{ player.life }}</span>
                      <button type="button" class="flex h-8 w-8 items-center justify-center rounded-full text-lg font-bold" style="background: rgba(0,0,0,0.12); color: #000;" @click.stop="adjustLife(player.id, 1)">+</button>
                    </div>

                    <div class="flex min-h-[14px] items-center gap-2 text-[10px]" style="color: rgba(0,0,0,0.65);">
                      <span v-if="player.poison > 0">☠ {{ player.poison }}</span>
                      <span v-for="opponent in opponentsOf(player)" :key="opponent.id">
                        <span v-if="(player.commanderDamage[opponent.id] ?? 0) > 0" class="inline-flex items-center gap-1">
                          <span class="h-1.5 w-1.5 rounded-full" :style="{ background: opponent.color }" />
                          {{ player.commanderDamage[opponent.id] }}
                        </span>
                      </span>
                    </div>

                    <button
                      type="button"
                      class="rounded-full border px-3.5 py-1 text-[10px] font-semibold"
                      style="background: rgba(255,255,255,0.2); border-color: rgba(0,0,0,0.3); color: rgba(0,0,0,0.75);"
                      @click.stop="nextTurn"
                    >
                      {{ $t('play.tracker.passTurn') }}
                    </button>
                  </div>

                  <div v-if="expandedPlayerId === player.id" class="absolute inset-0 flex items-center justify-center" style="background: rgba(5,3,8,0.97);" @click.stop="toggleExpand(player.id)">
                    <div class="flex flex-col items-center gap-1.5">
                      <p class="text-[9px] tracking-wide" style="color: #c4b5fd;">{{ $t('play.tracker.commanderDamage') }}</p>
                      <div v-for="opponent in opponentsOf(player)" :key="opponent.id" class="flex items-center gap-1.5">
                        <span class="h-2.5 w-2.5 rounded-full" :style="{ background: opponent.color }" />
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustCommanderDamage(player.id, opponent.id, -1)">−</button>
                        <span class="w-4 text-center text-[11px] tabular-nums" style="color: #fff;">{{ player.commanderDamage[opponent.id] ?? 0 }}</span>
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustCommanderDamage(player.id, opponent.id, 1)">+</button>
                      </div>
                      <div class="mt-1 flex items-center gap-1.5">
                        <span class="text-[9px]" style="color: #a78bfa;">{{ $t('play.tracker.poison') }}</span>
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustPoison(player.id, -1)">−</button>
                        <span class="w-4 text-center text-[11px] tabular-nums" style="color: #fff;">{{ player.poison }}</span>
                        <button type="button" class="flex h-[18px] w-[18px] items-center justify-center rounded-full text-[10px]" style="background: rgba(255,255,255,0.12); color: #fff;" @click.stop="adjustPoison(player.id, 1)">+</button>
                      </div>
                    </div>
                  </div>

                  <div v-if="started && isEliminated(player)" class="absolute inset-0 flex items-center justify-center" style="background: rgba(0,0,0,0.9);">
                    <span class="text-[19px] font-light tracking-[2px] text-white">{{ $t('play.tracker.dead') }}</span>
                  </div>
                </div>
              </div>
            </div>

            <span
              v-if="started"
              class="pointer-events-none absolute inset-0 flex items-center justify-center text-[96px] font-extrabold"
              style="color: rgba(255,255,255,0.05);"
            >{{ turn }}</span>

            <button
              v-if="!started"
              type="button"
              :title="$t('play.tracker.startRandom')"
              class="absolute left-1/2 top-1/2 flex h-[72px] w-[72px] -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full shadow-[0_10px_30px_rgba(139,92,246,0.45)]"
              style="background: linear-gradient(135deg, #8b5cf6, #a855f7);"
              @click="handleBeginGame"
            >
              <span class="ml-1 text-[26px]" style="color: #0a0714;">▶</span>
            </button>

            <div v-if="showStarterBanner" class="absolute inset-0 flex items-center justify-center" style="background: rgba(5,3,8,0.97);">
              <p class="px-6 text-center text-2xl font-bold" style="color: #f1f0f6;">{{ $t('play.tracker.starterBanner', { name: starterName }) }}</p>
            </div>

            <button
              type="button"
              class="absolute left-3 top-3 flex h-9 w-9 items-center justify-center rounded-full border text-sm"
              style="background: rgba(20,16,38,0.96); border-color: rgba(255,255,255,0.15); color: #f1f0f6;"
              @click="backToSetup"
            >
              ✕
            </button>

            <button
              v-if="started"
              type="button"
              class="absolute right-3 top-3 rounded-full border px-4 py-2 text-[13px]"
              style="background: rgba(20,16,38,0.96); border-color: rgba(248,113,113,0.35); color: #f87171;"
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
