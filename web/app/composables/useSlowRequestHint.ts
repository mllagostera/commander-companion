/**
 * Flags an in-flight operation that's taking suspiciously long, so the UI can
 * explain the wait instead of looking frozen. The backend runs on a free-tier
 * host that spins down after ~15 min idle and can take up to ~50s to wake up
 * on the next request (see ADR-0015).
 */
export function useSlowRequestHint(delayMs = 4000) {
  const active = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null

  function start() {
    stop()
    timer = setTimeout(() => {
      active.value = true
    }, delayMs)
  }

  function stop() {
    active.value = false
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  onUnmounted(stop)

  return { active, start, stop }
}
