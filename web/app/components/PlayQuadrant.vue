<script setup lang="ts">
import type { LocalPlayer } from '~/composables/useLocalGame'

const props = defineProps<{
  player: LocalPlayer
  allPlayers: LocalPlayer[]
  rotated: boolean
  started: boolean
  highlighted: boolean
  expandedId: number | null
}>()

const emit = defineEmits<{
  toggleExpand: [id: number]
  adjustLife: [id: number, amount: number]
  adjustPoison: [id: number, amount: number]
  adjustCommanderDamage: [defenderId: number, attackerId: number, amount: number]
}>()

const opponents = computed(() => props.allPlayers.filter((p) => p.id !== props.player.id))
const expanded = computed(() => props.expandedId === props.player.id)
const eliminated = computed(() => props.started && isLocalPlayerEliminated(props.player))
const poisonLabel = computed(() => props.player.poison > 0 ? `☠ ${props.player.poison}` : '')

const gridMid = computed(() => Math.ceil(props.allPlayers.length / 2))
const expandTop = computed(() => props.allPlayers.slice(0, gridMid.value))
const expandBottom = computed(() => props.allPlayers.slice(gridMid.value))

const transformStyle = computed(() => `rotate(${props.rotated ? 180 : 0}deg) scale(${props.highlighted ? 1.06 : 1})`)
const ringStyle = computed(() => props.highlighted
  ? '0 0 0 4px rgba(255,255,255,0.9) inset, 0 0 26px rgba(255,255,255,0.55)'
  : 'none')

function toggleExpand() {
  if (!props.started) return
  emit('toggleExpand', props.player.id)
}
</script>

<template>
  <div
    class="relative flex-1 overflow-hidden rounded-[22px] transition-[transform,box-shadow] duration-150"
    :style="{ background: player.color, transform: transformStyle, boxShadow: ringStyle }"
  >
    <div class="pointer-events-none absolute inset-0" style="background: linear-gradient(to bottom, rgba(10,7,20,0.05), rgba(10,7,20,0.45));" />

    <div v-if="!started" class="relative flex h-full w-full items-center justify-center">
      <span class="text-[13px] font-semibold" style="color: rgba(0,0,0,0.8);">{{ player.name }}</span>
    </div>

    <template v-else>
      <button type="button" class="absolute left-0 top-0 z-[1] h-full w-1/2" @click="emit('adjustLife', player.id, -1)" />
      <button type="button" class="absolute right-0 top-0 z-[1] h-full w-1/2" @click="emit('adjustLife', player.id, 1)" />
      <span
        class="pointer-events-none absolute top-1/2 -translate-y-1/2 font-light leading-none"
        style="left: clamp(10px, 4%, 22px); font-size: clamp(36px, 7vw, 64px); color: rgba(0,0,0,0.4);"
      >−</span>
      <span
        class="pointer-events-none absolute top-1/2 -translate-y-1/2 font-light leading-none"
        style="right: clamp(10px, 4%, 22px); font-size: clamp(36px, 7vw, 64px); color: rgba(0,0,0,0.4);"
      >+</span>

      <div class="pointer-events-none relative z-[2] flex h-full w-full flex-col items-center justify-between p-2.5">
        <span
          class="pointer-events-auto rounded-full font-bold"
          style="background: rgba(255,255,255,0.32); color: rgba(0,0,0,0.8); padding: clamp(6px,1.2vw,10px) clamp(14px,2.5vw,20px); font-size: clamp(12px,1.6vw,16px); min-height: 36px;"
        >{{ player.name }}</span>

        <span class="font-bold leading-none" style="font-size: clamp(30px, 6vw, 52px); color: #000;">{{ player.life }}</span>

        <button
          type="button"
          class="pointer-events-auto flex flex-col items-center gap-[3px] rounded-[14px] p-1.5"
          style="background: rgba(255,255,255,0.32);"
          @click="toggleExpand"
        >
          <div class="flex gap-[3px]">
            <span
              v-for="opponent in opponents"
              :key="opponent.id"
              class="relative flex items-center justify-center rounded-lg font-bold"
              style="width: clamp(26px,4vw,36px); height: clamp(26px,4vw,36px); background: rgba(255,255,255,0.55); font-size: clamp(13px,1.8vw,17px); color: rgba(0,0,0,0.8);"
            >
              <span class="absolute left-0.5 top-0.5 h-1.5 w-1.5 rounded-full" :style="{ background: opponent.color }" />
              {{ player.commanderDamage[opponent.id] ?? 0 }}
            </span>
          </div>
          <span v-if="poisonLabel" style="font-size: clamp(10px,1.4vw,13px); color: rgba(0,0,0,0.6);">{{ poisonLabel }}</span>
        </button>
      </div>
    </template>

    <div
      v-if="expanded"
      class="absolute inset-0 z-[3] flex items-center justify-center"
      style="background: rgba(5,3,8,0.55);"
      @click="toggleExpand"
    >
      <div class="flex flex-col items-center gap-1.5" @click.stop>
        <p class="text-[9px] uppercase tracking-wide" style="color: #c4b5fd;">{{ $t('play.tracker.commanderDamage') }}</p>

        <div v-for="row in [expandTop, expandBottom]" :key="row === expandTop ? 'top' : 'bottom'" class="flex gap-1.5">
          <div
            v-for="cell in row"
            :key="cell.id"
            class="flex items-center justify-center rounded-xl"
            style="width: clamp(48px,7vw,66px); height: clamp(48px,7vw,66px);"
            :style="{ background: cell.color }"
          >
            <div v-if="cell.id === player.id" class="flex flex-col items-center">
              <span class="h-3.5 w-3.5 rounded-full" style="background: rgba(0,0,0,0.55);" />
              <span class="-mt-0.5 h-3 w-6 rounded-t-xl" style="background: rgba(0,0,0,0.55);" />
            </div>
            <div v-else class="flex items-center gap-[3px]">
              <button
                type="button"
                class="flex items-center justify-center rounded-full"
                style="width: clamp(18px,2.6vw,24px); height: clamp(18px,2.6vw,24px); background: rgba(0,0,0,0.2); color: #000; font-size: clamp(12px,1.6vw,15px);"
                @click="emit('adjustCommanderDamage', player.id, cell.id, -1)"
              >−</button>
              <span class="font-bold" style="font-size: clamp(14px,2vw,18px); color: #000;">{{ player.commanderDamage[cell.id] ?? 0 }}</span>
              <button
                type="button"
                class="flex items-center justify-center rounded-full"
                style="width: clamp(18px,2.6vw,24px); height: clamp(18px,2.6vw,24px); background: rgba(0,0,0,0.2); color: #000; font-size: clamp(12px,1.6vw,15px);"
                @click="emit('adjustCommanderDamage', player.id, cell.id, 1)"
              >+</button>
            </div>
          </div>
        </div>

        <div class="mt-1 flex items-center gap-1.5">
          <span class="text-[9px]" style="color: #a78bfa;">{{ $t('play.tracker.poison') }}</span>
          <button
            type="button"
            class="flex items-center justify-center rounded-full"
            style="width: clamp(26px,4vw,36px); height: clamp(26px,4vw,36px); background: rgba(255,255,255,0.16); color: #fff; font-size: clamp(13px,1.8vw,17px);"
            @click="emit('adjustPoison', player.id, -1)"
          >−</button>
          <span class="w-[22px] text-center" style="font-size: clamp(13px,1.6vw,16px); color: #fff;">{{ player.poison }}</span>
          <button
            type="button"
            class="flex items-center justify-center rounded-full"
            style="width: clamp(26px,4vw,36px); height: clamp(26px,4vw,36px); background: rgba(255,255,255,0.16); color: #fff; font-size: clamp(13px,1.8vw,17px);"
            @click="emit('adjustPoison', player.id, 1)"
          >+</button>
        </div>
      </div>
    </div>

    <div v-if="eliminated" class="absolute inset-0 z-[3]" style="background: rgba(10,0,0,0.72);">
      <div class="absolute inset-0 animate-[cc-elim-flash_1.6s_ease-in-out_infinite]" style="background: radial-gradient(circle, transparent 38%, rgba(220,20,20,0.55) 100%);" />
      <div class="relative flex h-full flex-col items-center justify-center gap-1">
        <span class="leading-none" style="font-size: clamp(48px,8vw,72px); filter: drop-shadow(0 3px 0 #000);">💀</span>
        <span
          class="font-extrabold"
          style="font-size: clamp(18px,2.6vw,26px); letter-spacing: 2px; color: #fff; text-shadow: 2px 2px 0 #000, -1px -1px 0 #000; transform: rotate(-2deg);"
        >{{ $t('play.tracker.dead') }}</span>
      </div>
    </div>
  </div>
</template>
