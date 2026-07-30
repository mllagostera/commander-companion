<script setup lang="ts">
import type { LocalPlayer } from '~/composables/useLocalGame'

const {
  players, turn, isFinished, winnerId,
  setup, adjustLife, adjustPoison, adjustCommanderDamage, nextTurn, finishManually, reset,
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
const expandedCommanderDamage = ref<number | null>(null)

function toggleCommanderDamage(playerId: number) {
  expandedCommanderDamage.value = expandedCommanderDamage.value === playerId ? null : playerId
}

function opponentsOf(player: LocalPlayer): LocalPlayer[] {
  return players.value.filter((p) => p.id !== player.id)
}

const winner = computed<LocalPlayer | null>(() => players.value.find((p) => p.id === winnerId.value) ?? null)
</script>

<template>
  <div class="flex flex-col gap-6">
    <section v-if="phase === 'setup'" class="mx-auto flex w-full max-w-md flex-col gap-6">
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

    <section v-else class="flex flex-col gap-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <button
          type="button"
          class="rounded-full border px-4 py-2 text-[13px]"
          style="border-color: var(--input-border); color: var(--text);"
          @click="nextTurn"
        >
          {{ $t('play.tracker.turn', { n: turn }) }}
        </button>
        <button
          v-if="!isFinished"
          type="button"
          class="rounded-full border px-4 py-2 text-[13px]"
          style="border-color: rgba(248,113,113,0.35); background: var(--lose-bg); color: var(--lose);"
          @click="finishManually"
        >
          {{ $t('play.tracker.finish') }}
        </button>
      </div>

      <div v-if="isFinished" class="rounded-[24px] border p-6 text-center" style="border-color: var(--card-border); background: var(--card-bg);">
        <p class="text-xs uppercase tracking-wide" style="color: var(--text-dim);">{{ $t('play.summary.heading') }}</p>
        <p class="mt-2 text-xl font-semibold" :style="{ color: winner ? 'var(--win)' : 'var(--text)' }">
          {{ winner ? $t('play.summary.winner', { name: winner.name }) : $t('play.summary.draw') }}
        </p>
        <div class="mt-4 flex justify-center gap-3">
          <button
            type="button"
            class="rounded-full px-5 py-2.5 text-[13px] font-semibold text-[#0a0714]"
            style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
            @click="backToSetup"
          >
            {{ $t('play.summary.newGame') }}
          </button>
          <NuxtLink
            to="/"
            class="rounded-full border px-5 py-2.5 text-[13px]"
            style="border-color: var(--input-border); color: var(--text);"
          >
            {{ $t('play.summary.backHome') }}
          </NuxtLink>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="player in players"
          :key="player.id"
          class="flex flex-col gap-3 rounded-[22px] border-t-4 border p-4"
          :style="{ borderTopColor: player.color, borderColor: 'var(--card-border)', background: 'var(--card-bg)', opacity: isEliminated(player) ? 0.5 : 1 }"
        >
          <div class="flex items-center justify-between">
            <p class="font-semibold">{{ player.name }}</p>
            <span v-if="isEliminated(player)" class="rounded-full px-2 py-0.5 text-[11px] font-semibold" style="background: var(--lose-bg); color: var(--lose);">
              {{ $t('play.tracker.eliminated') }}
            </span>
          </div>

          <div class="flex items-center justify-between">
            <button
              type="button"
              class="h-10 w-10 rounded-full border text-lg"
              style="border-color: var(--input-border); color: var(--text);"
              @click="adjustLife(player.id, -1)"
            >
              −
            </button>
            <span class="text-3xl font-bold tabular-nums">{{ player.life }}</span>
            <button
              type="button"
              class="h-10 w-10 rounded-full border text-lg"
              style="border-color: var(--input-border); color: var(--text);"
              @click="adjustLife(player.id, 1)"
            >
              +
            </button>
          </div>

          <div class="flex items-center justify-between text-[13px]" style="color: var(--text-muted);">
            <span>{{ $t('play.tracker.poison') }}: {{ player.poison }}</span>
            <span class="flex gap-1.5">
              <button type="button" class="h-6 w-6 rounded-full border text-xs" style="border-color: var(--input-border);" @click="adjustPoison(player.id, -1)">−</button>
              <button type="button" class="h-6 w-6 rounded-full border text-xs" style="border-color: var(--input-border);" @click="adjustPoison(player.id, 1)">+</button>
            </span>
          </div>

          <button
            type="button"
            class="text-left text-xs"
            style="color: var(--accent-link);"
            @click="toggleCommanderDamage(player.id)"
          >
            {{ expandedCommanderDamage === player.id ? $t('play.tracker.hideCommanderDamage') : $t('play.tracker.showCommanderDamage') }}
          </button>

          <div v-if="expandedCommanderDamage === player.id" class="flex flex-col gap-1.5 border-t pt-2.5" style="border-color: var(--card-border);">
            <div v-for="opponent in opponentsOf(player)" :key="opponent.id" class="flex items-center justify-between text-xs">
              <span style="color: var(--text-muted);">{{ opponent.name }}</span>
              <span class="flex items-center gap-1.5">
                <button type="button" class="h-6 w-6 rounded-full border" style="border-color: var(--input-border);" @click="adjustCommanderDamage(player.id, opponent.id, -1)">−</button>
                <span class="w-4 text-center tabular-nums">{{ player.commanderDamage[opponent.id] ?? 0 }}</span>
                <button type="button" class="h-6 w-6 rounded-full border" style="border-color: var(--input-border);" @click="adjustCommanderDamage(player.id, opponent.id, 1)">+</button>
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
