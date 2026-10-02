// Captures the Play Store phone screenshots from the Android app running on
// an emulator, against a local backend seeded with seed.mjs. Elements are
// located through `uiautomator dump` (by visible text or content
// description) instead of fixed coordinates, so layout changes don't
// silently tap the wrong thing.
//
// Usage (see README.md for the full sequence, including the AVD):
//   node scripts/screenshots/capture-android.mjs [step...]
// With no steps it runs them all; naming steps runs just those (handy
// while adjusting one screen).

import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { DEMO_EMAIL, DEMO_PASSWORD } from './demo.mjs'

const PKG = 'com.vansid.tapeandocartones'
const ADB = process.env.ADB
  ?? path.join(process.env.LOCALAPPDATA ?? path.join(process.env.HOME ?? '', 'Library/Android'), 'Android/Sdk/platform-tools/adb')
const OUT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../android/store/screenshots/phone')

const sleep = ms => new Promise(r => setTimeout(r, ms))

function adb(...args) {
  return execFileSync(ADB, args, { maxBuffer: 64 * 1024 * 1024 })
}
const shell = (...args) => adb('shell', ...args).toString()

/** Every node of the current screen with its text, description and centre. */
function dump() {
  shell('uiautomator', 'dump', '/sdcard/ui.xml')
  const xml = adb('exec-out', 'cat', '/sdcard/ui.xml').toString('utf8')
  return [...xml.matchAll(/<node [^>]*>/g)].map(([node]) => {
    const attr = name => node.match(new RegExp(`${name}="([^"]*)"`))?.[1] ?? ''
    const [x1, y1, x2, y2] = attr('bounds').match(/\d+/g).map(Number)
    return {
      text: attr('text'), desc: attr('content-desc'), cls: attr('class'),
      clickable: attr('clickable') === 'true', x: (x1 + x2) >> 1, y: (y1 + y2) >> 1, x1, y1, x2, y2,
    }
  })
}

async function find(match, { timeout = 15000 } = {}) {
  const test = typeof match === 'string' ? n => n.text === match || n.desc === match : match
  const end = Date.now() + timeout
  for (;;) {
    const node = dump().find(test)
    if (node) return node
    if (Date.now() > end) throw new Error(`not found on screen: ${match}`)
    await sleep(700)
  }
}

const tap = ({ x, y }) => shell('input', 'tap', String(x), String(y))
async function tapOn(match, opts) { tap(await find(match, opts)); await sleep(900) }
const back = () => shell('input', 'keyevent', 'KEYCODE_BACK')

async function type(fieldIndex, value) {
  const fields = dump().filter(n => n.cls.endsWith('EditText'))
  tap(fields[fieldIndex])
  await sleep(500)
  shell('input', 'text', value)
  shell('input', 'keyevent', 'KEYCODE_ESCAPE') // closes the keyboard without leaving the screen
  await sleep(700)
}

async function save(name) {
  await sleep(1500) // let images and animations settle
  mkdirSync(OUT, { recursive: true })
  writeFileSync(path.join(OUT, `${name}.png`), adb('exec-out', 'screencap', '-p'))
  console.log(`saved ${name}.png`)
}

/** Clean status bar (10:00, full battery, no notifications), as Play recommends. */
function demoStatusBar() {
  shell('settings', 'put', 'global', 'sysui_demo_allowed', '1')
  const demo = (...extra) => shell('am', 'broadcast', '-a', 'com.android.systemui.demo', '-e', 'command', ...extra)
  demo('enter')
  demo('clock', '-e', 'hhmm', '1000')
  demo('battery', '-e', 'level', '100', '-e', 'plugged', 'false')
  demo('network', '-e', 'wifi', 'show', '-e', 'level', '4', '-e', 'fully', 'true')
  demo('network', '-e', 'mobile', 'hide')
  demo('notifications', '-e', 'visible', 'false')
}

async function restartApp() {
  shell('am', 'force-stop', PKG)
  shell('monkey', '-p', PKG, '-c', 'android.intent.category.LAUNCHER', '1')
  await sleep(4000)
}

const steps = {
  async login() {
    shell('pm', 'clear', PKG)
    shell('cmd', 'locale', 'set-app-locales', PKG, '--locales', 'es-ES')
    await restartApp()
    await find('Iniciar sesión')
    await type(0, DEMO_EMAIL)
    await type(1, DEMO_PASSWORD)
    await tapOn('Iniciar sesión')
    await find('NUEVA PARTIDA', { timeout: 30000 })
    await save('06-home')
  },

  async statistics() {
    await tapOn('Estadísticas')
    await find('RESUMEN GENERAL')
    await save('02-statistics')

    // Per-deck section further down, sorted by win rate so the best deck leads.
    shell('input', 'swipe', '540', '1700', '540', '400', '600')
    await tapOn('% victorias')
    await save('03-decks')
    back()
    await find('NUEVA PARTIDA')
  },

  async friends() {
    await tapOn('Amigos')
    await find('TUS AMIGOS')
    await save('05-friends')
    back()
    await find('NUEVA PARTIDA')
  },

  // A group game at "Mesa del Jueves": each seat gets a real member and deck,
  // so the tracker paints every quadrant with that commander's art.
  async tracker() {
    await tapOn('NUEVA PARTIDA')
    await tapOn('Grupo')
    await tapOn('Mesa del Jueves')
    await tapOn('EMPEZAR PARTIDA')
    landscape(true)
    await find('Asiento 1')

    const seats = [['chandra (tú)', 'Atraxa Superfriends'], ['ana', 'Korvold Treasures'], ['bruno', 'Muldrotha Value'], ['carla', 'Edgar Vampires']]
    for (const [i, [member, deck]] of seats.entries()) {
      await tapOn(inSeat(i, member))
      await tapOn(inSeat(i, deck))
    }
    await save('04-seats')
    await tapOn('EMPEZAR')
    await sleep(2000)

    // Mid-game state: seats 0 chandra, 1 ana, 2 bruno, 3 carla.
    await dealDamage(3, 0, 9)
    await dealDamage(2, 1, 8, { commander: true })
    await dealDamage(3, 1, 12)
    await dealDamage(0, 1, 11, { commander: true })
    await dealDamage(0, 2, 7, { commander: true })
    await dealDamage(1, 2, 11)
    await dealDamage(1, 3, 6, { commander: true })
    await dealDamage(2, 3, 7)
    for (let i = 0; i < 4; i++) await tapOn('PASAR TURNO') // back to chandra, turn 2
    await sleep(6000) // let the turn clock tick
    await save('01-tracker')

    // Leave the game (pause menu → finish → back home) and return to portrait.
    tap({ x: 1080, y: 540 })
    await tapOn('Finalizar partida')
    await tapOn('Volver al inicio')
    landscape(false)
    await find('NUEVA PARTIDA')
  },
}

// Seat centres in the landscape 2x2 grid, offset away from the life total
// and the pass-turn bar so the drag starts on the seat background.
const SEAT_POINTS = [[300, 440], [1300, 440], [300, 850], [1300, 850]]

/** Drags from one seat to another, then sets the amount in the damage dialog. */
async function dealDamage(from, to, amount, { commander = false } = {}) {
  const [[x1, y1], [x2, y2]] = [SEAT_POINTS[from], SEAT_POINTS[to]]
  shell('input', 'swipe', String(x1), String(y1), String(x2), String(y2), '900')
  await find('ASIGNAR DAÑO')
  if (commander) await tapOn('Daño de comandante')
  const plus = await find('Sumar 1')
  for (let i = 1; i < amount; i++) tap(plus)
  await tapOn('Aplicar')
}

/** The tracker's 2x2 seat grid in landscape: 0 top-left, 1 top-right, 2 bottom-left, 3 bottom-right. */
function inSeat(index, text) {
  return (n) => {
    if (n.text !== text && n.desc !== text) return false
    const [w, h] = [2160, 1080]
    const right = n.x > w / 2
    const bottom = n.y > h / 2
    return index === (bottom ? 2 : 0) + (right ? 1 : 0)
  }
}

function landscape(on) {
  shell('settings', 'put', 'system', 'accelerometer_rotation', '0')
  shell('settings', 'put', 'system', 'user_rotation', on ? '1' : '0')
}

const wanted = process.argv.slice(2)
demoStatusBar()
for (const [name, step] of Object.entries(steps)) {
  if (wanted.length && !wanted.includes(name)) continue
  console.log(`-- ${name}`)
  await step()
}
