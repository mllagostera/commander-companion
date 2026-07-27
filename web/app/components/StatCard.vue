<script setup lang="ts">
defineProps<{ label: string; value: number | string }>()
</script>

<template>
  <div class="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
    <p class="text-xs uppercase tracking-wide text-slate-500">{{ label }}</p>
    <p class="mt-1 text-2xl font-semibold tabular-nums">{{ value }}</p>
  </div>
</template>
