/**
 * Global confirmation toast ("Group created", "Session closed", etc.).
 * Additive to each form's inline messages, not a replacement: a specific form's
 * errors and successes still show right there, the toast is for
 * actions that don't have "a place" of their own on screen to show in.
 */
export function useToast() {
  const message = useState<string | null>('cc-toast', () => null)
  let timer: ReturnType<typeof setTimeout> | undefined

  function showToast(text: string) {
    message.value = text
    clearTimeout(timer)
    timer = setTimeout(() => {
      message.value = null
    }, 2600)
  }

  function dismissToast() {
    clearTimeout(timer)
    message.value = null
  }

  return { message, showToast, dismissToast }
}
