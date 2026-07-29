<script setup lang="ts">
const props = defineProps<{ played: number; won: number }>()

const label = computed(() => winRate(props.played, props.won))
const deg = computed(() => {
  if (!props.played) return 0
  return Math.round((props.won / props.played) * 360)
})
</script>

<template>
  <div
    class="relative flex h-[88px] w-[88px] flex-shrink-0 items-center justify-center rounded-full"
    :style="{
      background: `conic-gradient(var(--win) 0deg, var(--win) ${deg}deg, rgba(255,255,255,0.08) ${deg}deg, rgba(255,255,255,0.08) 360deg)`,
    }"
  >
    <div class="flex h-[58px] w-[58px] items-center justify-center rounded-full" style="background: var(--page-solid);">
      <span class="text-base font-bold" style="color: var(--win);">{{ label }}</span>
    </div>
  </div>
</template>
