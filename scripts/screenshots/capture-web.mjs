// Captures the landing page screenshots (web/public/landing/) from a running
// web client seeded with seed.mjs: the dashboard and the local life tracker,
// at 1440x900, Spanish, dark theme (the app's defaults).
//
// Usage (see README.md for the full sequence):
//   WEB_URL=http://localhost:3000 node scripts/screenshots/capture-web.mjs

import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { chromium } from 'playwright'
import sharp from 'sharp'
import { DEMO_EMAIL, DEMO_PASSWORD } from './demo.mjs'

const WEB = process.env.WEB_URL ?? 'http://localhost:3000'
const OUT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../web/public/landing')
const VIEWPORT = { width: 1440, height: 900 }

async function save(page, name) {
  // animations: 'disabled' fast-forwards finite CSS animations and resets
  // infinite ones, so no frame is caught mid-transition.
  const png = await page.screenshot({ animations: 'disabled' })
  await sharp(png).webp({ quality: 82 }).toFile(path.join(OUT, `${name}.webp`))
  console.log(`saved ${name}.webp`)
}

async function login(page) {
  await page.goto(`${WEB}/login`)
  await page.getByLabel(/email/i).fill(DEMO_EMAIL)
  await page.getByLabel(/contraseña/i).fill(DEMO_PASSWORD)
  await page.getByRole('button', { name: /^iniciar sesión$/i }).click()
  await page.waitForURL(`${WEB}/`, { waitUntil: 'commit' })
}

async function dashboard(page) {
  await page.goto(`${WEB}/`)
  await page.waitForLoadState('networkidle')
  // Deck art comes from cards.scryfall.io; wait until every image has decoded.
  await page.waitForFunction(() => [...document.images].every(img => img.complete && img.naturalWidth > 0))
  await save(page, 'dashboard')
}

async function lifeTracker(page) {
  await page.goto(`${WEB}/play`)
  await page.getByRole('button', { name: '4', exact: true }).click()
  for (const [i, name] of ['Chandra', 'Ana', 'Bruno', 'Carla'].entries()) {
    await page.getByLabel(`Jugador ${i + 1}`, { exact: true }).fill(name)
  }
  await page.getByRole('button', { name: 'Empezar partida' }).click()
  await page.getByRole('button', { name: 'Sortear quién empieza' }).click()
  // The draw animates for a few seconds, then shows "<name> empieza".
  await page.getByText(/ empieza$/).waitFor({ state: 'visible', timeout: 15000 }).catch(() => {})
  await page.getByText(/ empieza$/).waitFor({ state: 'hidden', timeout: 15000 }).catch(() => {})

  const quadrant = name => page.locator('div').filter({ has: page.getByRole('button', { name: `Restar 1 de vida a ${name}`, exact: true }) }).last()

  // Commander damage, set from the expanded panel of the player receiving it.
  // Each "+" adds 1 from that cell's opponent; the panel lists opponents in seat order.
  const commanderDamage = async (name, hitsPerOpponent) => {
    await quadrant(name).getByRole('button').filter({ hasText: /^\s*0/ }).first().click()
    const plus = page.getByRole('button', { name: '+', exact: true })
    for (const [i, hits] of hitsPerOpponent.entries()) {
      for (let n = 0; n < hits; n++) await plus.nth(i).click()
    }
    await page.keyboard.press('Escape')
    await quadrant(name).click({ position: { x: 20, y: 20 } })
  }
  await commanderDamage('Ana', [4, 8, 3])
  await commanderDamage('Carla', [0, 5, 1])

  for (const [name, hits] of [['Chandra', 15], ['Ana', 17], ['Bruno', 21], ['Carla', 12]]) {
    const minus = page.getByRole('button', { name: `Restar 1 de vida a ${name}`, exact: true })
    for (let n = 0; n < hits; n++) await minus.click()
  }
  await page.mouse.move(0, 0)
  await page.waitForTimeout(800)
  await save(page, 'life-tracker')
}

const browser = await chromium.launch()
const context = await browser.newContext({ viewport: VIEWPORT, locale: 'es-ES', colorScheme: 'dark' })
await context.addCookies([{ name: 'cc_locale', value: 'es', url: WEB }])
const page = await context.newPage()
try {
  await login(page)
  await dashboard(page)
  await lifeTracker(page)
}
finally {
  await browser.close()
}
