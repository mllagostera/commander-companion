/**
 * Toast global de confirmación ("Grupo creado", "Sesión cerrada", etc.).
 * Aditivo a los mensajes inline de cada formulario, no un reemplazo: errores
 * y éxitos de un form puntual se siguen mostrando ahí mismo, el toast es para
 * acciones que no tienen "un lugar" propio en la pantalla donde mostrarse.
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
