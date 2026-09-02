#!/usr/bin/env bash
# Fails when docs/database/schema.dbml has drifted from what the goose
# migrations actually produce.
#
# Why: docs-ci.yml already compiles the DBML to SQL, but that only proves it is
# well-formed -- not that it describes the real schema. Nothing caught the
# `deck_resync_jobs` table being entirely absent from schema.dbml for weeks
# (see TASKS.md, Stage 2). README §3 says "edit schema.dbml first", which only
# means anything if something verifies the two agree.
#
# Method: build the schema twice in one Postgres -- once by applying the goose
# migrations, once by applying `dbml2sql --postgres` -- and diff what
# information_schema reports for each.
#
# ------------------------------------------------------------------- SCOPE
# Compares tables, columns, data types and nullability. Deliberately NOT
# indexes or constraints, because the two sides cannot be made to agree there
# for reasons that have nothing to do with drift:
#
#   - DBML cannot express partial indexes. Five of this schema's unique indexes
#     carry a WHERE clause (`decks` on moxfield_id, `friend_requests` on
#     pending, the two job tables on active status, `tournament_participants`
#     on user_id) and dbml2sql emits them without the predicate.
#   - A UNIQUE *constraint* in a migration and a `[unique]` index in DBML are
#     the same guarantee but land in different information_schema tables, so
#     comparing constraints reports differences that are not differences.
#   - Index sort order (DESC) has no DBML syntax at all.
#
# So a missing or mistyped column fails this check; a missing index does not.
# That is the gap that actually bit, and a check with false positives gets
# switched off.

set -euo pipefail

DBML_FILE="${DBML_FILE:-docs/database/schema.dbml}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-backend/migrations}"
PGURL="${PGURL:-postgres://postgres:postgres@localhost:5432}"

FROM_MIGRATIONS=schema_check_migrations
FROM_DBML=schema_check_dbml

psql_do() { psql -q -v ON_ERROR_STOP=1 "$@"; }

echo "==> Rebuilding both schemas from scratch"
psql_do "$PGURL/postgres" -c "DROP DATABASE IF EXISTS $FROM_MIGRATIONS" -c "CREATE DATABASE $FROM_MIGRATIONS"
psql_do "$PGURL/postgres" -c "DROP DATABASE IF EXISTS $FROM_DBML" -c "CREATE DATABASE $FROM_DBML"

echo "==> Applying goose migrations"
goose -dir "$MIGRATIONS_DIR" postgres "$PGURL/$FROM_MIGRATIONS?sslmode=disable" up >/dev/null

echo "==> Compiling $DBML_FILE and applying it"
npx --yes -p @dbml/cli dbml2sql "$DBML_FILE" --postgres -o /tmp/schema-from-dbml.sql
psql_do "$PGURL/$FROM_DBML" -f /tmp/schema-from-dbml.sql >/dev/null

# goose_db_version is goose's own bookkeeping table and has no business being
# in the documented schema.
read -r -d '' QUERY <<'SQL' || true
SELECT table_name || '.' || column_name || ' : ' || data_type
       || CASE WHEN is_nullable = 'NO' THEN ' NOT NULL' ELSE '' END
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name <> 'goose_db_version'
ORDER BY table_name, column_name;
SQL

psql -q -A -t "$PGURL/$FROM_MIGRATIONS" -c "$QUERY" > /tmp/from-migrations.txt
psql -q -A -t "$PGURL/$FROM_DBML" -c "$QUERY" > /tmp/from-dbml.txt

echo "==> Comparing ($(wc -l < /tmp/from-migrations.txt) columns from migrations, $(wc -l < /tmp/from-dbml.txt) from DBML)"

if diff -u --label "from migrations" /tmp/from-migrations.txt --label "from schema.dbml" /tmp/from-dbml.txt; then
  echo ""
  echo "schema.dbml matches the migrations."
  exit 0
fi

cat <<'EOF'

schema.dbml and the migrations disagree.

Lines marked "-" exist in the real schema and are missing or wrong in the
DBML; lines marked "+" are documented but not real. Update
docs/database/schema.dbml to match -- it is a source of truth (README §3),
not a sketch.
EOF
exit 1
