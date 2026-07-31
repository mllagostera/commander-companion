export type Theme = 'dark' | 'light'

const STORAGE_KEY = 'cc-theme'

/**
 * Light/dark theme, persisted in localStorage and applied as
 * `data-theme` on <html> (see the custom properties in assets/css/main.css).
 * Only makes sense on the client — SSR always renders the dark default,
 * the real value is applied on mount to avoid a post-hydration flash that would be
 * more noticeable than a theme jump on the first paint.
 */
export function useTheme() {
  const theme = useState<Theme>('cc-theme', () => 'dark')

  function apply(value: Theme) {
    if (import.meta.client) {
      document.documentElement.setAttribute('data-theme', value)
    }
  }

  function setTheme(value: Theme) {
    theme.value = value
    apply(value)
    if (import.meta.client) {
      localStorage.setItem(STORAGE_KEY, value)
    }
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  function initTheme() {
    if (!import.meta.client) return
    const stored = localStorage.getItem(STORAGE_KEY)
    setTheme(stored === 'light' ? 'light' : 'dark')
  }

  return { theme, setTheme, toggleTheme, initTheme }
}
