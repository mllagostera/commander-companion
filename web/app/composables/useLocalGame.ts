// Life tracker 100% local para la web: sin llamadas al backend de partidas ni
// persistencia (ver ADR de alcance en docs/roadmap/TASKS.md, Stage 4b — "crear
// partidas desde la web"). Mismas reglas que el modo Casual de Android
// (GameState.kt/GameViewModel.kt): vida, turno, veneno y daño de comandante por
// oponente, con las mismas 3 condiciones de eliminación.
export const LOCAL_GAME_STARTING_LIFE = 40
export const COMMANDER_DAMAGE_LETHAL = 21
export const POISON_LETHAL = 10

/** Paleta fija de colores por asiento — no hace falta que el usuario elija un hex libre. */
export const LOCAL_PLAYER_COLORS = [
  '#8b5cf6', // violeta (marca de la app)
  '#3b82f6', // azul
  '#22c55e', // verde
  '#ef4444', // rojo
  '#eab308', // amarillo
  '#94a3b8', // gris (incoloro)
] as const

export interface LocalPlayer {
  id: number
  name: string
  color: string
  life: number
  poison: number
  /** Daño de comandante recibido, por id de oponente. */
  commanderDamage: Record<number, number>
}

export function isLocalPlayerEliminated(player: LocalPlayer): boolean {
  return (
    player.life <= 0
    || player.poison >= POISON_LETHAL
    || Object.values(player.commanderDamage).some((d) => d >= COMMANDER_DAMAGE_LETHAL)
  )
}

export function useLocalGame() {
  const players = ref<LocalPlayer[]>([])
  const turn = ref(1)
  const isFinished = ref(false)
  const winnerId = ref<number | null>(null)

  function setup(names: string[]) {
    players.value = names.map((name, i) => ({
      id: i,
      name: name.trim() || `P${i + 1}`,
      color: LOCAL_PLAYER_COLORS[i % LOCAL_PLAYER_COLORS.length]!,
      life: LOCAL_GAME_STARTING_LIFE,
      poison: 0,
      commanderDamage: {},
    }))
    turn.value = 1
    isFinished.value = false
    winnerId.value = null
  }

  function findPlayer(id: number): LocalPlayer | undefined {
    return players.value.find((p) => p.id === id)
  }

  function adjustLife(id: number, amount: number) {
    if (isFinished.value) return
    const player = findPlayer(id)
    if (!player) return
    player.life += amount
    checkGameOver()
  }

  function adjustPoison(id: number, amount: number) {
    if (isFinished.value) return
    const player = findPlayer(id)
    if (!player) return
    player.poison = Math.max(0, player.poison + amount)
    checkGameOver()
  }

  function adjustCommanderDamage(defenderId: number, attackerId: number, amount: number) {
    if (isFinished.value) return
    const defender = findPlayer(defenderId)
    if (!defender) return
    const current = defender.commanderDamage[attackerId] ?? 0
    defender.commanderDamage[attackerId] = Math.max(0, current + amount)
    if (amount > 0) defender.life -= amount
    checkGameOver()
  }

  function nextTurn() {
    if (isFinished.value) return
    turn.value += 1
  }

  function alivePlayers(): LocalPlayer[] {
    return players.value.filter((p) => !isLocalPlayerEliminated(p))
  }

  function checkGameOver() {
    const alive = alivePlayers()
    if (players.value.length > 1 && alive.length <= 1) {
      isFinished.value = true
      winnerId.value = alive[0]?.id ?? null
    }
  }

  /** Finalización manual (botón "Finalizar"), sin esperar a que quede 1 solo jugador. */
  function finishManually() {
    isFinished.value = true
    const alive = alivePlayers()
    winnerId.value = alive.length === 1 ? alive[0]!.id : null
  }

  function reset() {
    players.value = []
    turn.value = 1
    isFinished.value = false
    winnerId.value = null
  }

  return {
    players,
    turn,
    isFinished,
    winnerId,
    setup,
    adjustLife,
    adjustPoison,
    adjustCommanderDamage,
    nextTurn,
    finishManually,
    reset,
    isEliminated: isLocalPlayerEliminated,
  }
}
