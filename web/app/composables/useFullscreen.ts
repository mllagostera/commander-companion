/**
 * Real browser fullscreen (Fullscreen API), not the CSS `fixed inset-0` trick
 * used elsewhere for the tracker layout. Tracks `document.fullscreenElement`
 * via the `fullscreenchange` event so `isFullscreen` stays correct even when
 * the user exits with Esc or the browser's own UI instead of our button.
 */
export function useFullscreen() {
  const isFullscreen = ref(false)
  const isSupported = ref(false)

  function syncState() {
    isFullscreen.value = !!document.fullscreenElement
  }

  async function toggleFullscreen() {
    if (!import.meta.client || !isSupported.value) return
    if (document.fullscreenElement) {
      await document.exitFullscreen()
    } else {
      await document.documentElement.requestFullscreen()
    }
  }

  onMounted(() => {
    if (!import.meta.client) return
    isSupported.value = !!document.documentElement.requestFullscreen
    document.addEventListener('fullscreenchange', syncState)
    syncState()
  })

  onUnmounted(() => {
    if (!import.meta.client) return
    document.removeEventListener('fullscreenchange', syncState)
  })

  return { isFullscreen, isSupported, toggleFullscreen }
}
