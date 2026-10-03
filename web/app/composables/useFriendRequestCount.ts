/** How often the badge re-checks while the tab is visible and the user stays on one page. */
const POLL_INTERVAL_MS = 2 * 60 * 1000

/**
 * Pending incoming friend requests, shown as a badge on the "Friends" nav link
 * (layouts/default.vue). Shared through `useState` so the friends page can set it
 * from the list it already loaded, without an extra request.
 *
 * There's no push channel for this: the layout refreshes it on navigation, when the
 * tab becomes visible again, and every couple of minutes while it stays visible.
 */
export function useFriendRequestCount() {
  const count = useState<number>('friends-incoming-count', () => 0)
  const { countIncomingRequests } = useFriends()

  /** Best-effort: a failed check keeps the last known value instead of surfacing an error. */
  async function refresh() {
    try {
      count.value = (await countIncomingRequests()).incoming
    } catch {
      // The nav badge isn't worth an error message; the next check will try again.
    }
  }

  /**
   * Keeps the count fresh for as long as the calling component (the layout) is
   * mounted: on mount, on every route change, and periodically while visible.
   */
  function watchForChanges() {
    const route = useRoute()
    let timer: ReturnType<typeof setInterval> | undefined

    function onVisibilityChange() {
      if (document.visibilityState === 'visible') refresh()
    }

    onMounted(() => {
      refresh()
      document.addEventListener('visibilitychange', onVisibilityChange)
      timer = setInterval(() => {
        if (document.visibilityState === 'visible') refresh()
      }, POLL_INTERVAL_MS)
    })
    onUnmounted(() => {
      document.removeEventListener('visibilitychange', onVisibilityChange)
      clearInterval(timer)
    })
    watch(() => route.path, () => refresh())
  }

  return { count, refresh, watchForChanges }
}
