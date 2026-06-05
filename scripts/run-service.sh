#!/usr/bin/env bash
# =============================================================================
# Local dev runner — preflight + `go run ./cmd/<service>`
# =============================================================================
# Usage:
#   ./scripts/run-service.sh <service-name>
#
# Examples:
#   ./scripts/run-service.sh ws-gateway
#   ./scripts/run-service.sh orderbook-consumer
#
# What it does:
#   1. Verifies cmd/<service>/.env exists; copies from .env.example if not.
#   2. Loads env vars and validates the per-service required set.
#   3. TCP-probes NATS and Redis to fail fast before launching the binary.
#   4. (When redis-cli is present) PINGs Redis and reports DBSIZE for the
#      DBs the service will use, so you can spot empty caches early.
#   5. Execs `go run ./cmd/<service>` with the validated env.
# =============================================================================

set -euo pipefail

SERVICE="${1:-}"
if [ -z "$SERVICE" ]; then
  cat >&2 <<EOF
usage: $0 <service-name>

Supported services:
  ws-gateway              multi-channel WebSocket gateway + REST snapshot
  orderbook-consumer      Type 16+15 → orderbook engine → Redis db 9 + NATS
EOF
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CMD_DIR="$REPO_ROOT/cmd/$SERVICE"
ENV_FILE="$CMD_DIR/.env"
EXAMPLE_FILE="$CMD_DIR/.env.example"

# ---- pretty output ----------------------------------------------------------
B="\033[1m"; G="\033[32m"; Y="\033[33m"; R="\033[31m"; D="\033[2m"; N="\033[0m"
say()  { printf "${B}>> %s${N}\n" "$*"; }
ok()   { printf "${G}  ✓${N} %s\n" "$*"; }
warn() { printf "${Y}  ⚠${N} %s\n" "$*"; }
err()  { printf "${R}  ✗${N} %s\n" "$*" >&2; }
dim()  { printf "${D}     %s${N}\n" "$*"; }

[ -d "$CMD_DIR" ] || { err "cmd/$SERVICE not found in repo"; exit 1; }

# ---- per-service config -----------------------------------------------------
# REQUIRED is the list of env vars that MUST be set non-empty.
# REDIS_DB_VARS is the list of "VAR_NAME:label" pairs whose Redis DB index
# should be probed for DBSIZE during the preflight (e.g. "ORDERBOOK_REDIS_DB:orderbook").
case "$SERVICE" in
  ws-gateway)
    REQUIRED=(NATS_URL REDIS_ADDR)
    REDIS_DB_VARS=(
      "CANDLE_REDIS_DB:candle"
      "ORDERBOOK_REDIS_DB:orderbook"
    )
    ;;
  orderbook-consumer)
    REQUIRED=(NATS_URL REDIS_ADDR)
    REDIS_DB_VARS=("REDIS_DB:orderbook")
    ;;
  *)
    warn "no preflight profile for '$SERVICE' — using minimal checks (NATS_URL only)"
    REQUIRED=(NATS_URL)
    REDIS_DB_VARS=()
    ;;
esac

# ---- Step 1: .env ------------------------------------------------------------
say "Step 1/4: .env file"
if [ ! -f "$ENV_FILE" ]; then
  if [ -f "$EXAMPLE_FILE" ]; then
    warn ".env missing — copying from .env.example"
    cp "$EXAMPLE_FILE" "$ENV_FILE"
    ok "created $ENV_FILE — review + edit before re-running if needed"
    dim "(continuing with default values from .env.example)"
  else
    err ".env not found and no .env.example to copy from at $CMD_DIR"
    exit 1
  fi
else
  ok ".env present at $ENV_FILE"
fi

# Load .env into this shell. `set -a` exports every assignment.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

# ---- Step 2: required env vars ----------------------------------------------
say "Step 2/4: required env vars"
missing=0
for var in "${REQUIRED[@]}"; do
  if [ -z "${!var:-}" ]; then
    err "$var is not set"
    missing=$((missing + 1))
  else
    dim "$var = ${!var}"
  fi
done
if [ "$missing" -gt 0 ]; then
  err "$missing required var(s) missing — edit $ENV_FILE"
  exit 1
fi
ok "${#REQUIRED[@]} required var(s) set"

# ---- helpers ----------------------------------------------------------------
# parse_hostport "host:port" or "scheme://host:port" → echoes "<host> <port>"
parse_hostport() {
  local raw="$1" rest host port
  rest="${raw#*://}"          # strip scheme if present
  rest="${rest%%/*}"           # strip path if present
  host="${rest%:*}"
  port="${rest##*:}"
  if [ "$host" = "$port" ]; then
    # No colon — fall back to defaults per caller's hint
    port=""
  fi
  echo "$host $port"
}

tcp_probe() {
  local host="$1" port="$2"
  # bash's /dev/tcp redirect works on macOS + Linux; 2s connect timeout via `nc -z`.
  if command -v nc >/dev/null 2>&1; then
    nc -z -w 2 "$host" "$port" >/dev/null 2>&1
  else
    (exec 3<>"/dev/tcp/$host/$port") >/dev/null 2>&1 && exec 3<&-
  fi
}

# ---- Step 3: NATS + Redis reachability --------------------------------------
say "Step 3/4: connectivity"

read -r NATS_HOST NATS_PORT <<<"$(parse_hostport "$NATS_URL")"
NATS_PORT="${NATS_PORT:-4222}"
if tcp_probe "$NATS_HOST" "$NATS_PORT"; then
  ok "NATS reachable at $NATS_HOST:$NATS_PORT"
else
  err "NATS unreachable at $NATS_HOST:$NATS_PORT (TCP connect failed)"
  exit 1
fi

if [ -n "${REDIS_ADDR:-}" ]; then
  read -r REDIS_HOST REDIS_PORT <<<"$(parse_hostport "$REDIS_ADDR")"
  REDIS_PORT="${REDIS_PORT:-6379}"
  if tcp_probe "$REDIS_HOST" "$REDIS_PORT"; then
    ok "Redis reachable at $REDIS_HOST:$REDIS_PORT"
  else
    err "Redis unreachable at $REDIS_HOST:$REDIS_PORT"
    exit 1
  fi

  if command -v redis-cli >/dev/null 2>&1; then
    REDIS_AUTH_ARG=()
    if [ -n "${REDIS_PASSWORD:-}" ]; then
      REDIS_AUTH_ARG=(-a "$REDIS_PASSWORD" --no-auth-warning)
    fi
    if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" "${REDIS_AUTH_ARG[@]}" PING \
        | grep -q PONG; then
      ok "Redis AUTH + PING ok"
    else
      err "Redis PING failed — check REDIS_PASSWORD"
      exit 1
    fi
    for entry in "${REDIS_DB_VARS[@]:-}"; do
      [ -z "$entry" ] && continue
      var="${entry%%:*}"
      label="${entry##*:}"
      db="${!var:-}"
      if [ -z "$db" ]; then
        dim "skip $var ($label) — not set"
        continue
      fi
      size=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" "${REDIS_AUTH_ARG[@]}" \
              -n "$db" DBSIZE 2>/dev/null || echo "?")
      dim "$label  (db $db) → DBSIZE=$size"
    done
  else
    warn "redis-cli not in PATH — skipping AUTH/PING/DBSIZE probes"
  fi
else
  warn "REDIS_ADDR not set — skipping Redis preflight"
fi

# ---- Step 4: run ------------------------------------------------------------
say "Step 4/4: launch"
dim "exec: go run ./cmd/$SERVICE   (Ctrl+C to stop)"
echo
cd "$REPO_ROOT"
exec go run "./cmd/$SERVICE"
