<script setup lang="ts">
const { message, dismissToast } = useToast()
</script>

<template>
  <Transition
    enter-active-class="transition duration-200 ease-out"
    enter-from-class="opacity-0 translate-y-2"
    leave-active-class="transition duration-150 ease-in"
    leave-to-class="opacity-0"
  >
    <div
      v-if="message"
      class="fixed bottom-6 right-6 z-20 flex items-center gap-3 rounded-full border border-white/10 px-[18px] py-[13px] text-sm shadow-[0_8px_30px_rgba(0,0,0,0.45)] backdrop-blur-xl"
      style="background: var(--toast-bg); color: var(--text);"
    >
      {{ message }}
      <button
        type="button"
        class="p-0 text-[13px]"
        style="color: var(--text-dim);"
        @click="dismissToast"
      >
        ✕
      </button>
    </div>
  </Transition>
</template>
