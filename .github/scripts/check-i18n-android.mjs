#!/usr/bin/env node
// Fails when an Android string resource is missing from a translation, or when
// a translation defines something the default locale doesn't.
//
// The failure mode here is quieter than on the web: Android falls back to the
// default locale, so a string missing from values-en shows up in Spanish to an
// English user. Nothing crashes, `lintDebug` passes, and the only way to notice
// is to run the app in that language and read it.
//
// A string the default locale deliberately doesn't translate (a brand name,
// say) is marked `tools:ignore="MissingTranslation"` in values/strings.xml --
// the same annotation Android Lint honours -- and is skipped here too.

import { readFileSync, existsSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

const RES_DIR = 'android/app/src/main/res'
const DEFAULT_DIR = 'values'

// name + the rest of the opening tag, so tools:ignore can be read off it.
const RESOURCE = /<(string|plurals|string-array)\s+([^>]*?)name="([^"]+)"([^>]*)>/g

function resourcesOf(file) {
  const xml = readFileSync(file, 'utf8')
  const all = new Map() // name -> { kind, ignoresMissingTranslation }
  for (const [, kind, before, name, after] of xml.matchAll(RESOURCE)) {
    const attrs = before + after
    all.set(name, {
      kind,
      ignored: /tools:ignore="[^"]*MissingTranslation[^"]*"/.test(attrs),
    })
  }
  return all
}

const defaultFile = join(RES_DIR, DEFAULT_DIR, 'strings.xml')
if (!existsSync(defaultFile)) {
  console.error(`Default resources not found at ${defaultFile}`)
  process.exit(1)
}

const translationDirs = readdirSync(RES_DIR)
  .filter((d) => d.startsWith('values-') && existsSync(join(RES_DIR, d, 'strings.xml')))

const base = resourcesOf(defaultFile)
const problems = []
let ignoredCount = 0

for (const [name, meta] of base) {
  if (meta.ignored) {
    ignoredCount++
    continue
  }
  for (const dir of translationDirs) {
    if (!resourcesOf(join(RES_DIR, dir, 'strings.xml')).has(name)) {
      problems.push(`${dir}/strings.xml is missing <${meta.kind} name="${name}">`)
    }
  }
}

// The other direction: a translation defining something the default doesn't is
// dead weight at best, and at worst a rename that was only half applied.
for (const dir of translationDirs) {
  for (const name of resourcesOf(join(RES_DIR, dir, 'strings.xml')).keys()) {
    if (!base.has(name)) {
      problems.push(`${dir}/strings.xml defines "${name}", which ${DEFAULT_DIR}/strings.xml doesn't`)
    }
  }
}

// Every R.string.* the code references has to exist in the default locale. The
// compiler catches this too, but only once it gets that far -- and this runs in
// seconds without a Gradle build.
const KOTLIN_SRC = 'android/app/src/main/java'
const referenced = new Set()
const walk = (dir) => {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) walk(full)
    else if (entry.name.endsWith('.kt')) {
      for (const [, name] of readFileSync(full, 'utf8').matchAll(/R\.string\.([a-zA-Z0-9_]+)/g)) {
        referenced.add(name)
      }
    }
  }
}
if (existsSync(KOTLIN_SRC)) walk(KOTLIN_SRC)

for (const name of referenced) {
  if (!base.has(name)) problems.push(`code references R.string.${name}, absent from ${DEFAULT_DIR}/strings.xml`)
}

console.log(`default:      ${DEFAULT_DIR}/strings.xml (${base.size} resources, ${ignoredCount} exempt)`)
console.log(`translations: ${translationDirs.join(', ')}`)
console.log(`referenced:   ${referenced.size} distinct R.string.* in Kotlin`)

if (problems.length) {
  console.error(`\n${problems.length} problem(s):`)
  for (const p of problems) console.error(`  - ${p}`)
  process.exit(1)
}

console.log('\nEvery resource is translated in every locale, and every R.string.* resolves.')
