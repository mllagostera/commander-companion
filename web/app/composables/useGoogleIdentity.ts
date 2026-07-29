const SCRIPT_ID = 'google-identity-script'

function loadScript(): Promise<void> {
  if (typeof window === 'undefined') return Promise.resolve()
  if (window.google?.accounts?.id) return Promise.resolve()

  const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null
  if (existing) {
    return new Promise((resolve) => existing.addEventListener('load', () => resolve()))
  }

  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.id = SCRIPT_ID
    script.src = 'https://accounts.google.com/gsi/client'
    script.async = true
    script.defer = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('No se pudo cargar Google Identity Services'))
    document.head.appendChild(script)
  })
}

export function useGoogleIdentity() {
  const config = useRuntimeConfig()

  async function renderButton(
    container: HTMLElement,
    onCredential: (idToken: string) => void,
    options: { theme?: 'outline' | 'filled_black' | 'filled_blue' } = {},
  ) {
    if (!config.public.googleClientId) return

    await loadScript()
    if (!window.google?.accounts?.id) return

    window.google.accounts.id.initialize({
      client_id: config.public.googleClientId,
      callback: (response) => onCredential(response.credential),
    })
    container.innerHTML = ''
    window.google.accounts.id.renderButton(container, {
      theme: options.theme ?? 'filled_black',
      size: 'large',
      shape: 'pill',
      text: 'continue_with',
      logo_alignment: 'left',
      width: container.clientWidth || 320,
    })
  }

  return { renderButton }
}
