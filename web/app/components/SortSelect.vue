<script setup lang="ts">
// Custom listbox that replaces a native <select> where the app needs the
// dropdown's *open* panel themed (a native <select> popup is painted by the
// OS/browser and can't be reliably restyled with CSS across browsers).
// Named selectLabel (not ariaLabel): binding a component prop as
// `:aria-label` is intercepted by Vue's native-attribute fallthrough
// instead of reaching this prop, so the two names must differ.
const props = defineProps<{
  modelValue: string
  options: { value: string, label: string }[]
  selectLabel: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const isOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const optionRefs = ref<HTMLButtonElement[]>([])

const selectedLabel = computed(() => props.options.find((o) => o.value === props.modelValue)?.label ?? '')

function close() {
  isOpen.value = false
}

// Used when the listbox closes without another element taking focus (picking
// an option, pressing Escape) so keyboard focus doesn't fall through to
// <body>. Not used for outside clicks — there, whatever the user clicked
// already has focus and pulling it back to the trigger would fight that.
function closeAndRefocus() {
  isOpen.value = false
  triggerRef.value?.focus()
}

function select(value: string) {
  emit('update:modelValue', value)
  closeAndRefocus()
}

function handleDocumentClick(event: MouseEvent) {
  const target = event.target as Node
  if (rootRef.value && !rootRef.value.contains(target)) close()
}

function handleButtonKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    isOpen.value = true
  }
}

function focusOption(index: number) {
  const count = optionRefs.value.length
  optionRefs.value[(index + count) % count]?.focus()
}

function handleListKeydown(event: KeyboardEvent) {
  const currentIndex = optionRefs.value.findIndex((el) => el === document.activeElement)
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    focusOption(currentIndex + 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    focusOption(currentIndex - 1)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    closeAndRefocus()
  }
}

watch(isOpen, async (open) => {
  if (!open) return
  await nextTick()
  const selectedIndex = props.options.findIndex((o) => o.value === props.modelValue)
  focusOption(selectedIndex === -1 ? 0 : selectedIndex)
})

onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => document.removeEventListener('click', handleDocumentClick))
</script>

<template>
  <div ref="rootRef" class="relative">
    <button
      ref="triggerRef"
      type="button"
      :aria-label="selectLabel"
      aria-haspopup="listbox"
      :aria-expanded="isOpen"
      class="flex items-center gap-2 rounded-full border px-4 py-2.5 text-[13px] transition-colors hover:bg-[var(--card-bg-strong)]"
      style="background: var(--input-bg); border-color: var(--input-border); color: var(--text);"
      @click="isOpen = !isOpen"
      @keydown="handleButtonKeydown"
    >
      {{ selectedLabel }}
      <span
        aria-hidden="true"
        class="text-[10px] transition-transform"
        :style="{ color: 'var(--text-muted)', transform: isOpen ? 'rotate(180deg)' : 'rotate(0deg)' }"
      >▾</span>
    </button>

    <template v-if="isOpen">
      <div class="fixed inset-0 z-[29]" @click="close" />
      <div
        role="listbox"
        :aria-label="selectLabel"
        tabindex="-1"
        class="absolute left-0 top-[calc(100%+8px)] z-30 flex min-w-[180px] flex-col gap-0.5 rounded-[var(--radius-md)] border p-1.5 shadow-[0_16px_36px_rgba(0,0,0,0.3)] backdrop-blur-xl"
        style="background: var(--menu-bg); border-color: var(--card-border);"
        @keydown="handleListKeydown"
      >
        <button
          v-for="option in options"
          :key="option.value"
          ref="optionRefs"
          type="button"
          role="option"
          :aria-selected="option.value === modelValue"
          class="rounded-[var(--radius-sm)] px-3 py-2 text-left text-[13px] transition-colors"
          :class="option.value === modelValue ? '' : 'hover:bg-[var(--card-bg)]'"
          :style="option.value === modelValue
            ? 'background: rgba(139,92,246,0.18); color: var(--text); font-weight: 600;'
            : 'color: var(--text-muted);'"
          @click="select(option.value)"
        >
          {{ option.label }}
        </button>
      </div>
    </template>
  </div>
</template>
