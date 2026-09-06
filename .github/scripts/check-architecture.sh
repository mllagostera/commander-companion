#!/bin/sh
# Checks the architectural invariants that ARCHITECTURE.md / PROJECT-STRUCTURE.md
# state in prose, plus the two documentation-upkeep rules the README states in
# prose. Everything here was previously "a rule someone had to remember".
#
# Why this exists: a 2026-09-03 audit found that every invariant with an
# automated check was at 100% compliance, while three rules that lived only in
# prose (link new ADRs from the README hub, keep TASKS.md's review date fresh,
# keep ARCHITECTURE.md's claims true) had silently drifted. Only agents work in
# this repo, and an agent starts each session cold: a rule it doesn't happen to
# re-read at the moment it breaks it simply doesn't happen. So the rules that
# matter are checks, and the documents point at the check.
#
#
# This script is one of four guardrails, and deliberately checks only what the
# other three cannot express:
#
#   depguard (backend/.golangci.yml)   - Go import rules. Owns "a handler must
#                                        not import a database driver".
#   Konsist  (android .../architecture) - Kotlin AST assertions run as unit
#                                        tests. Owns the Android layering rules
#                                        and the two ratchets on known debt.
#   eslint-plugin-boundaries +
#   no-restricted-imports (web/eslint.config.mjs)
#                                      - JS/TS import rules. Owns "app/ must not
#                                        import server/", both directions.
#   this script                        - everything that is not an import: SQL
#                                        inside a string, a URL in the web
#                                        client, routes vs the OpenAPI file,
#                                        markdown links, a date in TASKS.md.
#                                        Plus one import rule no tool can state:
#                                        a slice reaching into ANOTHER slice's
#                                        sqlc Queries, because Service and
#                                        Queries live in the same Go package.
#
# Nothing is checked in two places on purpose. If you add a rule here, first ask
# whether one of the three above can express it natively -- native rules give
# better messages, run in an already-required gate, and cannot drift from a
# grep's idea of the same rule.
#
# Two severities:
#   FAIL - binary, mechanical, and the fix is obvious. Blocks CI.
#   WARN - needs judgement (a one-line PR shouldn't fail over a date). Reported,
#          never blocking, so the check doesn't teach anyone to bump a date
#          without auditing anything.
#
# Runs in seconds, no Docker, no build. Invoked by architecture-ci.yml and by
# .githooks/pre-push.
#
# Usage: sh .github/scripts/check-architecture.sh

set -u

cd "$(git rev-parse --show-toplevel)" || exit 1

fail=0
warn=0
tmp="${TMPDIR:-/tmp}/arch-check-$$"
mkdir -p "$tmp"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT

report_fail() {
  fail=$((fail + 1))
  echo "FAIL  $1"
  [ $# -gt 1 ] && echo "      -> $2"
  return 0
}

report_warn() {
  warn=$((warn + 1))
  echo "WARN  $1"
  [ $# -gt 1 ] && echo "      -> $2"
  return 0
}

section() { echo; echo "== $1"; }

# Every non-generated, non-test Go file under backend/internal.
production_go_files() {
  find backend/internal -name '*.go' \
    ! -path 'backend/internal/testutil/*' \
    ! -name '*_test.go' \
    ! -name 'query.sql.go' \
    ! -name 'querier.go' \
    ! -name 'models.go' \
    ! -name 'db.go' 2>/dev/null | sort
}

# ---------------------------------------------------------------------------
# 1. Handlers never return a raw error to the client.
#    internal/common/errors.go defines DomainError/MapError; a bare `return err`
#    ships an internal error (and whatever it leaks) straight to the caller.
#
#    The other half of the Handler rule -- not importing a database driver -- is
#    NOT here: it moved to `depguard` in backend/.golangci.yml, which expresses
#    it natively as an import rule and rides the golangci-lint check that branch
#    protection already requires. Deliberately not duplicated, so the two cannot
#    drift apart.
# ---------------------------------------------------------------------------
section "Handlers map their errors"

for f in backend/internal/*/handler.go; do
  [ -f "$f" ] || continue
  if grep -n '^\s*return err$' "$f" > "$tmp/bare" 2>/dev/null && [ -s "$tmp/bare" ]; then
    report_fail "$f returns a raw error to the client ($(wc -l < "$tmp/bare" | tr -d ' ') line(s))" \
      "wrap it: return common.MapError(err)"
  fi
done

# ---------------------------------------------------------------------------
# 2. Slices compose through each other's Service, never through its Queries.
#    A slice reaching into another slice's generated Queries bypasses that
#    slice's business rules and couples it to another module's tables — the
#    thing the modular monolith (ADR-0010) exists to prevent. Importing another
#    slice's Service/DTOs is the supported way and stays allowed.
# ---------------------------------------------------------------------------
section "No slice reaches into another slice's Queries"

slices=$(ls -d backend/internal/*/ 2>/dev/null | sed 's|backend/internal/||; s|/$||')

for f in $(production_go_files); do
  owner=$(basename "$(dirname "$f")")
  for other in $slices; do
    [ "$other" = "$owner" ] && continue
    # sqlc emits `package gameactions` for the game-actions/ directory.
    pkg=$(echo "$other" | tr -d '-')
    if grep -qE "\b${pkg}\.(New\(|Queries\b)" "$f"; then
      report_fail "$f uses ${pkg}.Queries directly" \
        "go through ${other}'s Service instead; only that slice owns its tables"
    fi
  done
done

# ---------------------------------------------------------------------------
# 3. All SQL lives in query.sql, so sqlc owns the generated access layer.
# ---------------------------------------------------------------------------
section "SQL only in query.sql"

for f in $(production_go_files); do
  # Only flag SQL inside a string literal (a double quote or backtick right
  # before the keyword), so prose in a comment doesn't trip the check.
  if grep -nE '["`][[:space:]]*(SELECT |INSERT INTO |DELETE FROM |UPDATE [a-z_]+ SET )' "$f" > "$tmp/sql" 2>/dev/null && [ -s "$tmp/sql" ]; then
    report_fail "$f contains inline SQL" \
      "move the query to that slice's query.sql and run make generate-sql"
  fi
done

# ---------------------------------------------------------------------------
# 4. The web client never talks to the Go API directly.
#    ADR-0004 / PROJECT-STRUCTURE.md §3.1: only Nitro (server/) holds the token;
#    the browser goes through /api/backend/**. A direct call from app/ would put
#    a token in reach of client-side JS.
# ---------------------------------------------------------------------------
section "Web client goes through the Nitro BFF"

if [ -d web/app ]; then
  if grep -rnE 'apiBase|localhost:8080|/api/v1' web/app > "$tmp/web" 2>/dev/null && [ -s "$tmp/web" ]; then
    while IFS= read -r line; do
      report_fail "web/app calls the Go API directly: ${line%%:*}" \
        "use useApi()'s apiFetch, which goes through /api/backend/**"
    done < "$tmp/web"
  fi
fi

# ---------------------------------------------------------------------------
# 5. Every registered route is in the OpenAPI contract, and vice versa.
#    README §3: "if you're going to change how backend and clients communicate,
#    edit openapi.yaml first." This is what proves it happened.
#    /ws/** is a deliberate, recorded exception — see ADR-0005, which explains
#    why OpenAPI 3.1 doesn't model WebSockets and leaves it as a separate task.
#
#    common/health.go is in the list because GET /health is registered on the
#    Fiber app itself, not on a slice's router: it lives at the root, outside
#    /api/v1 (see ADR-0020). It used to be invisible to this check, which is
#    how it stayed out of the contract until the build marker was added to it.
# ---------------------------------------------------------------------------
section "Routes match openapi.yaml"

if [ -f docs/api/openapi.yaml ]; then
  : > "$tmp/code_paths"
  for f in backend/internal/*/handler.go backend/internal/websocket/*.go backend/internal/common/health.go; do
    [ -f "$f" ] || continue
    case "$f" in *_test.go) continue ;; esac
    slice=$(basename "$(dirname "$f")")
    prefix=""
    # main.go mounts admin under protected.Group("/admin", ...).
    [ "$slice" = "admin" ] && prefix="/admin"
    # `router.` is a slice registering under /api/v1; `app.` is a route on the
    # Fiber app itself, which is why both receivers are matched.
    grep -oE '(router|app)\.(Get|Post|Put|Patch|Delete)\("[^"]*"' "$f" 2>/dev/null \
      | sed "s|^[a-z]*\.[A-Za-z]*(\"|${prefix}|; s|\"$||" \
      >> "$tmp/code_paths"
  done
  # :id -> {id}, drop the WebSocket exception, dedupe.
  sed 's/:\([a-zA-Z][a-zA-Z0-9]*\)/{\1}/g' "$tmp/code_paths" \
    | grep -v '^/ws/' | sort -u > "$tmp/code_paths_norm"

  grep -E '^  /' docs/api/openapi.yaml | sed 's|^  ||; s|:[[:space:]]*$||' | sort -u > "$tmp/oas_paths"

  comm -23 "$tmp/code_paths_norm" "$tmp/oas_paths" > "$tmp/undocumented"
  comm -13 "$tmp/code_paths_norm" "$tmp/oas_paths" > "$tmp/phantom"

  while IFS= read -r p; do
    [ -n "$p" ] && report_fail "route $p is registered but absent from openapi.yaml" \
      "document it in docs/api/openapi.yaml — the contract is edited first, not last"
  done < "$tmp/undocumented"

  while IFS= read -r p; do
    [ -n "$p" ] && report_fail "openapi.yaml documents $p, which no handler registers" \
      "remove it from the contract, or register the route"
  done < "$tmp/phantom"
fi

# ---------------------------------------------------------------------------
# 6. Every document is reachable from the README hub.
#    README §8: "every document in the repo should be linked from here. If you
#    add a new document under docs/, add it to this list too in the same
#    change." A document nobody links is a document no future session reads.
# ---------------------------------------------------------------------------
section "Every doc is linked from the README hub"

for f in $(find docs -name '*.md' 2>/dev/null | sort); do
  # Normalize Windows-style separators just in case find returns them.
  path=$(echo "$f" | tr '\\' '/')
  if ! grep -qF "$path" README.md; then
    report_fail "$path is not linked from README.md" \
      "add it to the documentation hub (README §8), in this same change"
  fi
done

# ---------------------------------------------------------------------------
# 7. TASKS.md's review date tracks the code. WARNING, on purpose.
#    Blocking this would just teach an agent to bump the date without auditing
#    anything, which is worse than the drift it would hide.
# ---------------------------------------------------------------------------
section "TASKS.md review date (advisory)"

# 15 commits touching backend/web/android without a status audit is where
# TASKS.md stops being trustworthy as "the source of truth for real status".
STALE_COMMIT_THRESHOLD=15

if [ -f docs/roadmap/TASKS.md ]; then
  reviewed=$(grep -oE '\*\*Last reviewed:\*\*[[:space:]]*[0-9]{4}-[0-9]{2}-[0-9]{2}' docs/roadmap/TASKS.md \
    | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
  if [ -z "$reviewed" ]; then
    report_warn "TASKS.md has no parseable '**Last reviewed:** YYYY-MM-DD' line"
  else
    since=$(git log --since="$reviewed" --oneline -- backend web android 2>/dev/null | wc -l | tr -d ' ')
    if [ "$since" -gt "$STALE_COMMIT_THRESHOLD" ]; then
      report_warn "TASKS.md last reviewed $reviewed, $since code commits since" \
        "audit the affected stages, update the date, and add the DECISIONS-LOG.md entry"
    fi
  fi
fi

# ---------------------------------------------------------------------------

echo
if [ "$fail" -gt 0 ]; then
  echo "check-architecture: $fail failure(s), $warn warning(s)."
  echo "Each line above names the file and the fix. These are the invariants in"
  echo "docs/architecture/ARCHITECTURE.md and PROJECT-STRUCTURE.md, enforced."
  exit 1
fi

if [ "$warn" -gt 0 ]; then
  echo "check-architecture: all invariants hold, $warn warning(s) (not blocking)."
else
  echo "check-architecture: all invariants hold."
fi
exit 0
