import type { Ref } from 'vue'

/**
 * Wires up the dialog-accessibility behavior the app's custom modals were
 * missing: focus moves into the dialog when it opens (honoring an existing
 * `autofocus` element if there is one), Tab/Shift+Tab stay trapped inside it,
 * Escape closes it, and focus returns to whatever triggered it on close.
 * Doesn't touch markup or styling — pass the ref of the dialog's outer
 * container and this only adds behavior.
 */
export function useModalA11y(isOpen: Ref<boolean>, containerRef: Ref<HTMLElement | null>, onClose: () => void) {
  let previouslyFocused: HTMLElement | null = null

  function focusableElements(): HTMLElement[] {
    const root = containerRef.value
    if (!root) return []
    return Array.from(
      root.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      ),
    ).filter((el) => el.offsetParent !== null)
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
      return
    }
    if (event.key !== 'Tab') return

    const focusable = focusableElements()
    if (!focusable.length) return
    const first = focusable[0]!
    const last = focusable[focusable.length - 1]!

    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  watch(isOpen, async (open) => {
    if (open) {
      previouslyFocused = document.activeElement as HTMLElement | null
      await nextTick()
      const autofocusTarget = containerRef.value?.querySelector<HTMLElement>('[autofocus]')
      ;(autofocusTarget ?? focusableElements()[0])?.focus()
      document.addEventListener('keydown', handleKeydown)
    } else {
      document.removeEventListener('keydown', handleKeydown)
      previouslyFocused?.focus()
      previouslyFocused = null
    }
  })

  onUnmounted(() => document.removeEventListener('keydown', handleKeydown))
}
