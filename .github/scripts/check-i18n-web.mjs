#!/usr/bin/env node
// Fails when the web client references a translation key that doesn't resolve,
// or when the locales have drifted apart.
//
// Why this exists: a missing key is invisible to every other check. Vue
// renders the key itself, the page still returns HTTP 200, and eslint,
// vue-tsc and `nuxt build` all pass. That is exactly how /friends shipped
// showing a literal "friends.add.heading" to users (fixed in #101) -- it went
// through CI, a browser and a route smoke test without anyone noticing.
//
// Two checks:
//   1. Parity   -- every locale defines exactly the same set of keys.
//   2. Coverage -- every key used in the source resolves in every locale.

import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, extname } from 'node:path'

const LOCALES_DIR = 'web/i18n/locales'
const SOURCE_DIRS = ['web/app', 'web/server']
const SOURCE_EXTS = new Set(['.vue', '.ts', '.js', '.mjs'])

// `t('a.b')`, `$t("a.b")`, with or without further arguments. Keys built at
// runtime (`t(someVar)`, template literals) can't be resolved statically and
// are counted separately rather than reported as failures.
const KEY_CALL = /(?<![\w.])\$?t\(\s*(['"])([\w.-]+)\1/g
const DYNAMIC_CALL = /(?<![\w.])\$?t\(\s*(?!['"])[^)]/g

function flatten(value, prefix = '') {
  if (value === null || typeof value !== 'object') return [prefix]
  return Object.entries(value).flatMap(([k, v]) => flatten(v, prefix ? `${prefix}.${k}` : k))
}

function walk(dir) {
  const out = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) out.push(...walk(full))
    else if (SOURCE_EXTS.has(extname(entry))) out.push(full)
  }
  return out
}

const locales = readdirSync(LOCALES_DIR)
  .filter((f) => f.endsWith('.json'))
  .map((f) => {
    const name = f.replace(/\.json$/, '')
    const keys = new Set(flatten(JSON.parse(readFileSync(join(LOCALES_DIR, f), 'utf8'))))
    return { name, keys }
  })

if (locales.length === 0) {
  console.error(`No locale files found in ${LOCALES_DIR}`)
  process.exit(1)
}

const problems = []

// --- 1. parity -------------------------------------------------------------
// Compared against the union rather than against one designated "base" locale,
// so a key added only to a translation is reported too, not just a missing one.
const union = new Set(locales.flatMap((l) => [...l.keys]))
for (const locale of locales) {
  for (const key of union) {
    if (!locale.keys.has(key)) problems.push(`${locale.name}.json is missing key: ${key}`)
  }
}

// --- 2. coverage -----------------------------------------------------------
const files = SOURCE_DIRS.flatMap((d) => walk(d))
const used = new Map() // key -> first file that uses it
let dynamicCount = 0

for (const file of files) {
  const source = readFileSync(file, 'utf8')
  for (const match of source.matchAll(KEY_CALL)) {
    if (!used.has(match[2])) used.set(match[2], file)
  }
  dynamicCount += [...source.matchAll(DYNAMIC_CALL)].length
}

for (const [key, file] of used) {
  const missingIn = locales.filter((l) => !l.keys.has(key)).map((l) => l.name)
  if (missingIn.length) {
    problems.push(`${file} uses "${key}", missing in: ${missingIn.join(', ')}`)
  }
}

// --- report ----------------------------------------------------------------
console.log(`locales:        ${locales.map((l) => `${l.name} (${l.keys.size})`).join(', ')}`)
console.log(`source files:   ${files.length}`)
console.log(`keys used:      ${used.size} static, ${dynamicCount} built at runtime (not checkable)`)

if (problems.length) {
  console.error(`\n${problems.length} problem(s):`)
  for (const p of problems) console.error(`  - ${p}`)
  process.exit(1)
}

console.log('\nEvery key used resolves in every locale, and the locales match.')
