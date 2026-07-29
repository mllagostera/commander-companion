<script setup lang="ts">
withDefaults(defineProps<{ size?: 'sm' | 'lg'; pulse?: boolean }>(), {
  size: 'sm',
  pulse: false,
})
</script>

<template>
  <span
    class="relative flex-shrink-0"
    :class="[
      size === 'lg' ? 'w-[52px] h-[72px]' : 'w-[26px] h-[36px]',
      pulse ? 'animate-[cc-pulse_1.1s_ease-in-out_infinite]' : 'rotate-[-6deg]',
    ]"
  >
    <span
      class="absolute inset-0 rounded-[15%] shadow-[0_6px_20px_rgba(139,92,246,0.45)]"
      style="background: linear-gradient(160deg, #4c1d95, #2f2159);"
    />
    <span
      class="absolute inset-[8%] rounded-[12%] flex flex-col items-center bg-[#e9e4fb]"
      :class="size === 'lg' ? 'p-1' : 'p-0.5'"
    >
      <span class="w-full flex-1 rounded-[15%]" style="background: linear-gradient(135deg, #8b5cf6, #a855f7);" />
      <span
        class="font-extrabold tracking-wide text-[#2f2159] mt-0.5"
        :class="size === 'lg' ? 'text-[10px]' : 'text-[5px]'"
      >CC</span>
    </span>
  </span>
</template>
