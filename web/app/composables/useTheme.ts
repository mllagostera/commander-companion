export type Theme = 'dark' | 'light'

const STORAGE_KEY = 'cc-theme'

/**
 * Tema claro/oscuro, persistido en localStorage y aplicado como
 * `data-theme` en <html> (ver las custom properties en assets/css/main.css).
 * Solo tiene sentido en el cliente — SSR siempre renderiza el default oscuro,
 * el valor real se aplica en el mount para evitar un flash post-hidratación
 * más notorio que un salto de tema en el primer paint.
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
