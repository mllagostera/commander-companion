<script setup lang="ts">
withDefaults(
  defineProps<{ label: string; value: number | string; tint?: string; valueColor?: string }>(),
  { tint: '', valueColor: '' },
)
</script>

<template>
  <div
    class="rounded-[22px] border p-4"
    :style="{
      background: tint ? `linear-gradient(160deg, ${tint}, var(--card-bg))` : 'var(--card-bg)',
      borderColor: 'var(--card-border)',
    }"
  >
    <p class="text-[11px] uppercase tracking-wide" style="color: var(--text-muted);">{{ label }}</p>
    <p class="mt-1.5 text-2xl font-semibold tabular-nums" :style="{ color: valueColor || 'var(--text)' }">
      {{ value }}
    </p>
  </div>
</template>
