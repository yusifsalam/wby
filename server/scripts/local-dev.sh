#!/usr/bin/env bash
set -euo pipefail

SERVER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATION_FILE="$SERVER_DIR/migrations/001_initial.sql"

# Source .env if present (does not override existing env vars)
if [[ -f "$SERVER_DIR/.env" ]]; then
  set -a
  source "$SERVER_DIR/.env"
  set +a
fi

POSTGRES_FORMULA="${POSTGRES_FORMULA:-postgresql@18}"
DB_NAME="${DB_NAME:-wby}"
DB_USER="${DB_USER:-wby}"
DB_PASSWORD="${DB_PASSWORD:-wby}"
PORT="${PORT:-8080}"
DATABASE_URL="${DATABASE_URL:-}"
FMI_BASE_URL="${FMI_BASE_URL:-https://opendata.fmi.fi/wfs}"
FMI_API_KEY="${FMI_API_KEY:-}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/local-dev.sh up          # start local db (if needed), init schema, run server
  ./scripts/local-dev.sh init-db     # start local db (if needed), create role/db, apply schema
  ./scripts/local-dev.sh run-server  # run API server only
EOF
}

log() {
  printf '[local-dev] %s\n' "$*"
}

add_brew_pg_bin_to_path() {
  local brew_prefix
  if ! command -v brew >/dev/null 2>&1; then
    return
  fi

  brew_prefix="$(brew --prefix 2>/dev/null || true)"
  if [[ -n "$brew_prefix" && -d "$brew_prefix/opt/$POSTGRES_FORMULA/bin" ]]; then
    export PATH="$brew_prefix/opt/$POSTGRES_FORMULA/bin:$PATH"
  fi
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "Missing required command: $1"
    exit 1
  fi
}

configure_database_url() {
  if [[ -z "$DATABASE_URL" ]]; then
    DATABASE_URL="postgres://$DB_USER:$DB_PASSWORD@localhost:5432/$DB_NAME?sslmode=disable"
  fi
}

start_postgres_if_needed() {
  if pg_isready -q >/dev/null 2>&1; then
    return
  fi

  if command -v brew >/dev/null 2>&1 && brew list --formula "$POSTGRES_FORMULA" >/dev/null 2>&1; then
    log "Starting $POSTGRES_FORMULA with brew services..."
    brew services start "$POSTGRES_FORMULA" >/dev/null
    sleep 2
  fi

  if ! pg_isready -q >/dev/null 2>&1; then
    log "Postgres is not reachable. Start it manually, then re-run this script."
    exit 1
  fi
}

ensure_role_and_db() {
  log "Ensuring role '$DB_USER' exists..."
  psql postgres -v ON_ERROR_STOP=1 --set=db_user="$DB_USER" --set=db_password="$DB_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'db_user', :'db_password')
WHERE NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = :'db_user')\gexec
SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', :'db_user', :'db_password')\gexec
SQL

  log "Ensuring database '$DB_NAME' exists..."
  psql postgres -v ON_ERROR_STOP=1 --set=db_name="$DB_NAME" --set=db_user="$DB_USER" <<'SQL'
SELECT format('CREATE DATABASE %I OWNER %I', :'db_name', :'db_user')
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'db_name')\gexec
SELECT format('ALTER DATABASE %I OWNER TO %I', :'db_name', :'db_user')\gexec
SQL
}

apply_schema_if_needed() {
  log "Ensuring PostGIS extension is enabled..."
  if ! psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS postgis;" >/dev/null 2>&1; then
    log "App role cannot create extensions; retrying with local Postgres admin user..."
    if ! psql -d "$DB_NAME" -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS postgis;" >/dev/null; then
      log "Failed to create PostGIS extension. Run this once as a Postgres superuser:"
      log "psql -d \"$DB_NAME\" -c 'CREATE EXTENSION IF NOT EXISTS postgis;'"
      exit 1
    fi
  fi

  if [[ "$(psql "$DATABASE_URL" -tAc "SELECT to_regclass('public.stations') IS NOT NULL")" != "t" ]]; then
    log "Applying migration: $MIGRATION_FILE"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$MIGRATION_FILE" >/dev/null
  else
    log "Base schema already initialized."
  fi

  for migration in "$SERVER_DIR"/migrations/*.sql; do
    [[ -e "$migration" ]] || continue
    if [[ "$migration" == "$MIGRATION_FILE" ]]; then
      continue
    fi
    log "Applying migration: $migration"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$migration" >/dev/null
  done
}

run_server() {
  require_cmd go
  export PORT DATABASE_URL FMI_BASE_URL FMI_API_KEY
  log "Starting server on :$PORT"
  cd "$SERVER_DIR"
  exec go run ./cmd/server
}

main() {
  local cmd="${1:-up}"
  add_brew_pg_bin_to_path
  configure_database_url

  case "$cmd" in
    up)
      require_cmd psql
      require_cmd pg_isready
      start_postgres_if_needed
      ensure_role_and_db
      apply_schema_if_needed
      run_server
      ;;
    init-db)
      require_cmd psql
      require_cmd pg_isready
      start_postgres_if_needed
      ensure_role_and_db
      apply_schema_if_needed
      ;;
    run-server)
      run_server
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
