<script setup lang="ts">
/**
 * Initial-in-a-circle avatar. The backend exposes no avatar image, so a colour
 * from the shared palette stands in for one (see utils/avatarColors.ts).
 *
 * The colour is derived from the whole username rather than from its position
 * in a list, so the same person keeps the same colour wherever they appear —
 * a per-list index would give them a different one on each screen. (The
 * friends list previously keyed off `username.length`, which handed every
 * five-letter name the same colour.)
 */
const props = withDefaults(defineProps<{ username: string, size?: number }>(), { size: 32 })

const initial = computed(() => props.username?.[0]?.toUpperCase() ?? '?')

const paletteIndex = computed(() => {
  let sum = 0
  for (const char of props.username ?? '') sum += char.codePointAt(0) ?? 0
  return sum
})
</script>

<template>
  <span
    aria-hidden="true"
    class="flex flex-shrink-0 items-center justify-center rounded-full font-semibold text-[#0a0714]"
    :style="{
      width: `${size}px`,
      height: `${size}px`,
      fontSize: `${Math.round(size * 0.42)}px`,
      background: avatarColor(paletteIndex),
    }"
  >
    {{ initial }}
  </span>
</template>
