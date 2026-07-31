// Life tracker 100% local for the web: no calls to the games backend and no
// persistence (see the scope ADR in docs/roadmap/TASKS.md, Stage 4b — "create
// games from the web"). Same rules as Android's Casual mode
// (GameState.kt/GameViewModel.kt): life, turn, poison and commander damage per
// opponent, with the same 3 elimination conditions.
export const LOCAL_GAME_STARTING_LIFE = 40
export const COMMANDER_DAMAGE_LETHAL = 21
export const POISON_LETHAL = 10

/** Fixed color palette per seat — no need for the user to pick a free-form hex. */
export const LOCAL_PLAYER_COLORS = [
  '#8b5cf6', // violet (app brand color)
  '#3b82f6', // blue
  '#22c55e', // green
  '#ef4444', // red
  '#eab308', // yellow
  '#94a3b8', // gray (colorless)
] as const

export interface LocalPlayer {
  id: number
  name: string
  color: string
  life: number
  poison: number
  /** Commander damage received, keyed by opponent id. */
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
  /** Actual game start: kept separate from `setup()` so seats can be shown
   * before drawing who goes first (the central "Play" button in the tracker). */
  const started = ref(false)
  const startingPlayerId = ref<number | null>(null)

  /** Draw animation (roulette): highlights seats in sequence, slowing down, before
   * landing on the winner — replaces the previous instant draw. */
  const lotteryActive = ref(false)
  const lotteryHighlightId = ref<number | null>(null)
  const showStarterBanner = ref(false)
  /** Cancellation token: any game reset invalidates the draw's in-flight timeouts,
   * so they don't apply stale state if the user has already left the screen. */
  let lotteryToken = 0

  function setup(names: string[]) {
    lotteryToken += 1
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
    started.value = false
    startingPlayerId.value = null
    lotteryActive.value = false
    lotteryHighlightId.value = null
    showStarterBanner.value = false
  }

  /** Randomly draws who goes first with a roulette animation (cycles through the seats
   * slowing down) before announcing the winner with a ~1.8s banner. */
  function startRandomPlayer() {
    const order = players.value.map((p) => p.id)
    if (started.value || order.length === 0) return
    const finalIndex = Math.floor(Math.random() * order.length)
    const totalSteps = order.length * 3 + finalIndex + 1
    const token = (lotteryToken += 1)
    lotteryActive.value = true
    let step = 0
    const tick = () => {
      if (token !== lotteryToken) return
      lotteryHighlightId.value = order[step % order.length]!
      step += 1
      if (step < totalSteps) {
        setTimeout(tick, 70 + (step / totalSteps) * 200)
      } else {
        lotteryActive.value = false
        lotteryHighlightId.value = null
        startingPlayerId.value = order[finalIndex]!
        started.value = true
        showStarterBanner.value = true
        setTimeout(() => {
          if (token === lotteryToken) showStarterBanner.value = false
        }, 1800)
      }
    }
    tick()
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

  /** Manual finish (the "Finish" button), without waiting for only 1 player to remain. */
  function finishManually() {
    isFinished.value = true
    const alive = alivePlayers()
    winnerId.value = alive.length === 1 ? alive[0]!.id : null
  }

  function reset() {
    lotteryToken += 1
    players.value = []
    turn.value = 1
    isFinished.value = false
    winnerId.value = null
    started.value = false
    startingPlayerId.value = null
    lotteryActive.value = false
    lotteryHighlightId.value = null
    showStarterBanner.value = false
  }

  return {
    players,
    turn,
    isFinished,
    winnerId,
    started,
    startingPlayerId,
    lotteryActive,
    lotteryHighlightId,
    showStarterBanner,
    setup,
    startRandomPlayer,
    adjustLife,
    adjustPoison,
    adjustCommanderDamage,
    finishManually,
    reset,
    isEliminated: isLocalPlayerEliminated,
  }
}
