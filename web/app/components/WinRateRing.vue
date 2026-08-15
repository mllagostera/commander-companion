<script setup lang="ts">
const props = withDefaults(defineProps<{ played: number; won: number; size?: number }>(), { size: 88 })

const label = computed(() => winRate(props.played, props.won))
const deg = computed(() => {
  if (!props.played) return 0
  return Math.round((props.won / props.played) * 360)
})

// The hole is a fixed 30px narrower than the ring at every size, so the band
// keeps the same weight whether it's rendered at the default 88px (statistics)
// or larger (the dashboard's performance card).
const innerSize = computed(() => props.size - 30)
</script>

<template>
  <div
    class="relative flex flex-shrink-0 items-center justify-center rounded-full"
    :style="{
      width: `${size}px`,
      height: `${size}px`,
      background: `conic-gradient(var(--win) 0deg, var(--win) ${deg}deg, rgba(255,255,255,0.08) ${deg}deg, rgba(255,255,255,0.08) 360deg)`,
    }"
  >
    <div
      class="flex items-center justify-center rounded-full"
      :style="{ width: `${innerSize}px`, height: `${innerSize}px`, background: 'var(--page-solid)' }"
    >
      <span class="font-bold" :style="{ color: 'var(--win)', fontSize: size >= 104 ? '20px' : '16px' }">{{ label }}</span>
    </div>
  </div>
</template>
