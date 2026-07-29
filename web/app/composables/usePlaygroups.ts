import type { Playgroup, PlaygroupMember } from '~/types/api'

export function usePlaygroups() {
  const { apiFetch } = useApi()

  /** Grupos de los que el usuario autenticado es miembro, con `members` poblado. */
  function listPlaygroups() {
    return apiFetch<Playgroup[]>('/playgroups')
  }

  /** El creador queda como primer miembro automáticamente (lo hace el backend). */
  function createPlaygroup(name: string) {
    return apiFetch<Playgroup>('/playgroups', {
      method: 'POST',
      body: { name: name.trim() },
    })
  }

  /** Único camino con `members` poblado. 404 si no existe o no sos miembro. */
  function getPlaygroup(id: string) {
    return apiFetch<Playgroup>(`/playgroups/${id}`)
  }

  /** Solo un miembro existente puede renombrar el grupo (mismo criterio que addMember). */
  function updatePlaygroup(id: string, name: string) {
    return apiFetch<Playgroup>(`/playgroups/${id}`, {
      method: 'PATCH',
      body: { name: name.trim() },
    })
  }

  /** userId debe ser el UUID de un usuario ya existente; quien invita debe ya ser miembro. */
  function addMember(playgroupId: string, userId: string) {
    return apiFetch<PlaygroupMember>(`/playgroups/${playgroupId}/members`, {
      method: 'POST',
      body: { user_id: userId.trim() },
    })
  }

  return { listPlaygroups, createPlaygroup, getPlaygroup, updatePlaygroup, addMember }
}

export function createPlaygroupError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 400:
      return 'El grupo necesita un nombre.'
    default:
      return apiErrorMessage(err, 'No se pudo crear el grupo.')
  }
}

/**
 * 404 acá cubre tanto "el grupo no existe" como "no sos miembro" (el backend
 * no distingue, ver getMemberPlaygroup en internal/playgroups/service.go).
 */
export function getPlaygroupError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 404:
      return 'El grupo no existe o no sos miembro.'
    default:
      return apiErrorMessage(err, 'No se pudo cargar el grupo.')
  }
}

/** Mismo mapeo de errores que createPlaygroupError/getPlaygroupError: 400 sin nombre, 404 si no sos miembro. */
export function updatePlaygroupError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 400:
      return 'El grupo necesita un nombre.'
    case 404:
      return 'El grupo no existe o no sos miembro.'
    default:
      return apiErrorMessage(err, 'No se pudo renombrar el grupo.')
  }
}

/** Ver ErrInvalidUserID (400), ErrUserNotFound (404) y ErrAlreadyMember (409) en internal/playgroups/service.go. */
export function addMemberError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 400:
      return 'Ese ID de usuario no es válido.'
    case 404:
      return 'No existe un usuario con ese ID.'
    case 409:
      return 'Ese usuario ya es miembro del grupo.'
    default:
      return apiErrorMessage(err, 'No se pudo agregar al usuario.')
  }
}
