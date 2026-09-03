// Config generated/extended by the @nuxt/eslint module (see nuxt.config.ts).
// Nuxt's preset already knows about auto-imports, the `app/` srcDir, and server/.
import withNuxt from './.nuxt/eslint.config.mjs'
import boundaries from 'eslint-plugin-boundaries'

// Architectural guardrails for the web client.
//
// ADR-0004 and docs/architecture/PROJECT-STRUCTURE.md §3.1 draw one hard line:
// `server/` (Nitro, the BFF) is the ONLY layer that holds the session token and
// the only one that talks to the Go API; `app/` reaches it exclusively over
// HTTP through /api/backend/**. Importing server code into `app/` would bundle
// backend-only logic — and the module that reads the httpOnly cookie — into
// something the browser can load. That line was prose until now.
//
// Enforced twice on purpose, because the two mechanisms fail differently:
//
//   1. `boundaries/dependencies` understands the layer graph and catches
//      relative imports ('../../server/utils/backend'), including ones nobody
//      thought to enumerate. It relies on path resolution, so a Nuxt alias it
//      cannot resolve is silently skipped rather than flagged.
//   2. `no-restricted-imports` matches the alias spellings literally
//      ('~~/server/...', '#server/...'). No resolver involved, so it cannot be
//      defeated by a resolution gap — but it only knows the patterns listed.
//
// What neither can see: `app/` calling the Go API by URL instead of by import
// (a hardcoded 'http://localhost:8080' or a runtimeConfig.apiBase read). That is
// a string, not a dependency, and it stays checked by
// .github/scripts/check-architecture.sh.
const architectureBoundaries = {
  name: 'commander/architecture-boundaries',
  plugins: { boundaries },
  settings: {
    'boundaries/include': ['app/**/*', 'server/**/*', 'shared/**/*'],
    // eslint-plugin-boundaries resolves each import to a file before it can
    // classify it; without a TypeScript-aware resolver every .ts import comes
    // back unresolved and the rule silently passes. Found the hard way: the
    // rule reported nothing at all until this line existed.
    'import/resolver': { typescript: { project: './tsconfig.json' } },
    'boundaries/elements': [
      { type: 'nitro', pattern: 'server/**/*' },
      { type: 'client', pattern: 'app/**/*' },
      { type: 'shared', pattern: 'shared/**/*' },
    ],
  },
  rules: {
    'boundaries/dependencies': [
      'error',
      {
        default: 'disallow',
        policies: [
          // `shared/` is the neutral leaf: types both sides agree on. It must
          // stay dependency-free in both directions, or it stops being shared.
          {
            from: { element: { type: 'shared' } },
            allow: { to: { element: { type: 'shared' } } },
          },
          {
            from: { element: { type: 'client' } },
            allow: { to: { element: { types: { anyOf: ['client', 'shared'] } } } },
          },
          {
            from: { element: { type: 'nitro' } },
            allow: { to: { element: { types: { anyOf: ['nitro', 'shared'] } } } },
          },
        ],
      },
    ],
  },
}

const noServerImportsFromApp = {
  name: 'commander/no-server-imports-from-app',
  files: ['app/**/*.{ts,js,mjs,vue}'],
  rules: {
    'no-restricted-imports': [
      'error',
      {
        patterns: [
          {
            group: [
              '~~/server/**',
              '~/server/**',
              '@@/server/**',
              '@/server/**',
              '#server/**',
              '**/../server/**',
            ],
            message:
              'app/ must not import server/ — Nitro is the only layer that holds the session token and talks to the Go API. Call it over HTTP with useApi()\'s apiFetch (/api/backend/**) instead. See ADR-0004.',
          },
        ],
      },
    ],
  },
}

export default withNuxt(architectureBoundaries, noServerImportsFromApp)
