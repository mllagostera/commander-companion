// Seeds a fresh local stack with the demo data the store and landing
// screenshots show: five players with real commander art, two playgroups,
// ~22 finished games with a plausible history, friendships and a tournament
// halfway through round 2. Everything goes through the public API, so the
// statistics are the backend's own, not hand-written rows; only the game
// dates are rewritten afterwards (see backdate()), because games created in
// one run would otherwise all share the same minute.
//
// Usage (from the repo root, against an EMPTY database — see README.md):
//   node scripts/screenshots/seed.mjs
//
// Env: API_BASE (default http://localhost:8080/api/v1), COMPOSE_PROJECT
// (default tc-capture, the Compose project whose `db` service gets the
// backdating SQL).

import { execFileSync } from 'node:child_process'
import { DEMO_PASSWORD, demoEmail } from './demo.mjs'

const API = process.env.API_BASE ?? 'http://localhost:8080/api/v1'
const COMPOSE_PROJECT = process.env.COMPOSE_PROJECT ?? 'tc-capture'

const PLAYERS = {
  chandra: [
    ['Atraxa Superfriends', "Atraxa, Praetors' Voice"],
    ['Yuriko Ninjas', "Yuriko, the Tiger's Shadow"],
    ['Krenko Goblins', 'Krenko, Mob Boss'],
    ['Kaalia Reanimator', 'Kaalia of the Vast'],
  ],
  ana: [['Korvold Treasures', 'Korvold, Fae-Cursed King']],
  bruno: [['Muldrotha Value', 'Muldrotha, the Gravetide']],
  carla: [['Edgar Vampires', 'Edgar Markov']],
  dani: [['Miirym Dragons', 'Miirym, Sentinel Wyrm']],
}

// chandra's history, oldest first: [deck index, table, winner, days ago].
// The tail (three wins in a row) is what the dashboard's streak shows.
const GAMES = [
  [0, 'jueves', 'ana', 40], [1, 'liga', 'chandra', 38], [2, 'jueves', 'bruno', 36],
  [0, 'jueves', 'chandra', 34], [3, 'liga', 'dani', 31], [1, 'jueves', 'chandra', 29],
  [0, 'liga', 'chandra', 27], [2, 'jueves', 'carla', 24], [0, 'jueves', 'chandra', 22],
  [3, 'liga', 'chandra', 20], [1, 'jueves', 'ana', 17], [0, 'liga', 'chandra', 15],
  [2, 'jueves', 'chandra', 13], [3, 'liga', 'bruno', 10], [1, 'jueves', 'carla', 8],
  [0, 'liga', 'dani', 6], [3, 'jueves', 'carla', 5], [0, 'jueves', 'chandra', 3],
  [0, 'jueves', 'chandra', 2], [0, 'liga', 'chandra', 1],
]

const TABLES = {
  jueves: { name: 'Mesa del Jueves', members: ['chandra', 'ana', 'bruno', 'carla'] },
  liga: { name: 'Liga Central', members: ['chandra', 'bruno', 'carla', 'dani'] },
}

async function api(token, method, path, body) {
  const res = await fetch(`${API}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status} ${text}`)
  return text ? JSON.parse(text) : null
}

// Scryfall asks for a descriptive User-Agent and ~10 requests per second at most.
async function artCrop(commander) {
  await new Promise(r => setTimeout(r, 120))
  const res = await fetch(`https://api.scryfall.com/cards/named?exact=${encodeURIComponent(commander)}`, {
    headers: { 'User-Agent': 'TapeandoCartones-screenshot-seed/1.0', 'Accept': 'application/json' },
  })
  if (!res.ok) return undefined
  const card = await res.json()
  return card.image_uris?.art_crop ?? card.card_faces?.[0]?.image_uris?.art_crop
}

async function signUp(username) {
  const email = demoEmail(username)
  await api(null, 'POST', '/auth/register', { username, email, password: DEMO_PASSWORD, locale: 'es' })
  const login = await api(null, 'POST', '/auth/login', { email, password: DEMO_PASSWORD })
  return { username, email, token: login.access_token, id: login.user.id, decks: [] }
}

function pick(list, i) { return list[i % list.length] }

async function playGame(users, host, [deckIndex, tableKey, winner], seed) {
  const table = TABLES[tableKey]
  const group = table.playgroupId
  const game = await api(host.token, 'POST', '/games', { playgroup_id: group })

  const seats = {}
  for (const name of table.members) {
    const u = users[name]
    const deck = name === host.username ? u.decks[deckIndex] : u.decks[0]
    const body = name === host.username ? { deck_id: deck.id } : { deck_id: deck.id, user_id: u.id }
    seats[name] = (await api(host.token, 'POST', `/games/${game.id}/join`, body)).id
  }
  await api(host.token, 'POST', `/games/${game.id}/start`)

  const act = (actor, action_type, extra = {}) =>
    api(host.token, 'POST', `/games/${game.id}/actions`, { actor_id: seats[actor], action_type, ...extra })

  // A few rounds of turns, each with a timed TurnEnd and some damage, so
  // turn-time, damage and commander-damage statistics all have data.
  const order = table.members
  for (let round = 0; round < 3; round++) {
    for (const [i, name] of order.entries()) {
      await act(name, 'TurnStart')
      const victim = pick(order.filter(n => n !== name), seed + round + i)
      if ((seed + round + i) % 3 === 0) {
        await act(name, 'CommanderDamage', { target_id: seats[victim], payload: { amount: 3 + ((seed + i) % 5) } })
      }
      else {
        await act(name, 'CombatDamage', { target_id: seats[victim], payload: { amount: 2 + ((seed + round) % 6) } })
      }
      await act(name, 'TurnEnd', { payload: { duration_ms: 45000 + ((seed * 7919 + round * 104729 + i * 15485863) % 150000) } })
    }
  }

  // The winner knocks everyone else out; the last survivor wins (the
  // backend's sole-survivor rule).
  for (const loser of order.filter(n => n !== winner)) {
    await act(winner, 'Elimination', { target_id: seats[loser] })
  }
  await api(host.token, 'POST', `/games/${game.id}/finish`)
  return game.id
}

function backdate(gameDays) {
  const statements = gameDays.map(([id, days], i) => {
    // Evenings, between 19:00 and 21:00, so the history reads like game nights.
    // Evenings, between 19:00 and 21:00, lasting 70-120 minutes, with the
    // actions spread evenly across that span so the timeline reads in order.
    const minutes = 19 * 60 + (i * 37) % 120
    const length = 70 + (i * 23) % 50
    return `UPDATE games SET
  started_at = date_trunc('day', now()) - interval '${days} days' + interval '${minutes} minutes',
  created_at = date_trunc('day', now()) - interval '${days} days' + interval '${minutes - 5} minutes',
  finished_at = date_trunc('day', now()) - interval '${days} days' + interval '${minutes + length} minutes'
WHERE id = '${id}';
UPDATE game_actions a SET created_at = g.started_at + (g.finished_at - g.started_at) * (o.n::float / o.total)
FROM games g, (SELECT id, row_number() OVER (ORDER BY created_at, id) AS n, count(*) OVER () AS total
               FROM game_actions WHERE game_id = '${id}') o
WHERE g.id = '${id}' AND a.id = o.id;`
  })
  execFileSync('docker', ['compose', '-p', COMPOSE_PROJECT, 'exec', '-T', 'db', 'psql', '-q', '-U', 'postgres', '-d', 'commander', '-v', 'ON_ERROR_STOP=1'],
    { input: statements.join('\n'), stdio: ['pipe', 'inherit', 'inherit'] })
}

async function main() {
  const users = {}
  for (const name of Object.keys(PLAYERS)) {
    users[name] = await signUp(name)
    for (const [deckName, commander] of PLAYERS[name]) {
      const image_url = await artCrop(commander)
      users[name].decks.push(await api(users[name].token, 'POST', '/decks', { name: deckName, commander, image_url }))
    }
    console.log(`user ${name}: ${users[name].decks.length} deck(s)`)
  }
  const chandra = users.chandra

  for (const table of Object.values(TABLES)) {
    table.playgroupId = (await api(chandra.token, 'POST', '/playgroups', { name: table.name })).id
    for (const name of table.members.filter(n => n !== 'chandra')) {
      await api(chandra.token, 'POST', `/playgroups/${table.playgroupId}/members`, { user_id: users[name].id })
    }
  }

  for (const name of ['ana', 'bruno', 'carla']) {
    const req = await api(chandra.token, 'POST', '/friends/requests', { addressee_id: users[name].id })
    await api(users[name].token, 'POST', `/friends/requests/${req.id}/accept`)
  }
  await api(users.dani.token, 'POST', '/friends/requests', { addressee_id: chandra.id }) // left pending on purpose

  const gameDays = []
  for (const [i, g] of GAMES.entries()) {
    gameDays.push([await playGame(users, chandra, g, i), g[3]])
  }
  console.log(`games: ${gameDays.length} finished`)
  backdate(gameDays)

  // Tournament: four registered players plus four guests, round 1 played,
  // round 2 under way.
  const t = await api(chandra.token, 'POST', '/tournaments', { name: 'Torneo de Otoño', target_players: 8 })
  for (const name of ['chandra', 'ana', 'bruno', 'carla']) {
    await api(users[name].token, 'POST', '/tournaments/join', { join_code: t.join_code, deck_id: users[name].decks[0].id })
  }
  for (const [guest_name, commander_name] of [['Marc', 'Prosper, Tome-Bound'], ['Laia', "Sythis, Harvest's Hand"], ['Elena', 'Kenrith, the Returned King'], ['Dani', 'Miirym, Sentinel Wyrm']]) {
    await api(chandra.token, 'POST', `/tournaments/${t.id}/participants`, { guest_name, commander_name })
  }
  await api(chandra.token, 'POST', `/tournaments/${t.id}/start`)
  let detail = await api(chandra.token, 'GET', `/tournaments/${t.id}`)
  for (const table of detail.rounds.at(-1).tables) {
    const results = table.seats.map((s, i) => ({ participant_id: s.participant_id, finish_position: i + 1 }))
    await api(chandra.token, 'POST', `/tournaments/${t.id}/tables/${table.id}/result`, { results })
  }
  await api(chandra.token, 'POST', `/tournaments/${t.id}/rounds/next`)
  detail = await api(chandra.token, 'GET', `/tournaments/${t.id}`)
  console.log(`tournament: round ${detail.tournament.current_round} of ${detail.tournament.round_count}`)

  console.log(`\nDone. Log in as ${chandra.email} / ${DEMO_PASSWORD}`)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
