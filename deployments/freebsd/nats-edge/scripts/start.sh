#!/bin/sh
# Start edge nats-server under daemon(8) supervisor.
#
# Loads $NATS_TOKEN from $ROOT/conf/nats-edge.env (gitignored, mode 0600)
# and substitutes it into the config via -E (eval) — config file uses
# $NATS_TOKEN literal so the secret never lands in `ps`.
#
# Stop with: ./stop.sh

set -e

ROOT="$HOME/nats-edge"
BIN="$ROOT/bin/nats-server"
CONF_TEMPLATE="$ROOT/conf/nats-server.conf"
CONF_RENDERED="$ROOT/run/nats-server.rendered.conf"
ENVFILE="$ROOT/conf/nats-edge.env"
LOG="$ROOT/log/nats-server.log"
DPID="$ROOT/run/daemon.pid"
SPID="$ROOT/run/nats-server.pid"

if [ ! -x "$BIN" ]; then
    echo "ERROR: binary not found or not executable: $BIN" >&2
    echo "Run scripts/install-binary.sh first." >&2
    exit 1
fi
if [ ! -f "$CONF_TEMPLATE" ]; then
    echo "ERROR: config template not found: $CONF_TEMPLATE" >&2
    exit 1
fi
if [ ! -f "$ENVFILE" ]; then
    echo "ERROR: env file not found: $ENVFILE" >&2
    echo "Create it with NATS_TOKEN=... and LEAF_TOKEN=... (mode 0600)." >&2
    exit 1
fi

mkdir -p "$ROOT/data/jetstream" "$ROOT/log" "$ROOT/run"

if [ -f "$DPID" ] && kill -0 "$(cat "$DPID")" 2>/dev/null; then
    echo "Already running (daemon pid $(cat "$DPID"))."
    exit 0
fi

# Load env (only NATS_TOKEN should be there). `set -a` exports.
set -a
. "$ENVFILE"
set +a

if [ -z "$NATS_TOKEN" ]; then
    echo "ERROR: NATS_TOKEN empty after sourcing $ENVFILE" >&2
    exit 1
fi
if [ -z "$LEAF_TOKEN" ]; then
    echo "ERROR: LEAF_TOKEN empty after sourcing $ENVFILE" >&2
    exit 1
fi

: >> "$LOG"

# Render config: NATS HOCON parser tidak expand $VAR di dalam quoted strings,
# jadi kita substitusi manual di sini sebelum exec. Output ke run/ supaya
# token tidak masuk ke conf/ yang di-version-control.
sed -e "s|\$NATS_TOKEN|$NATS_TOKEN|g" \
    -e "s|\$LEAF_TOKEN|$LEAF_TOKEN|g" \
    "$CONF_TEMPLATE" > "$CONF_RENDERED"
chmod 600 "$CONF_RENDERED"

/usr/sbin/daemon \
    -r -R 5 \
    -P "$DPID" \
    -t nats-edge \
    -o "$LOG" \
    -f \
    "$BIN" --config "$CONF_RENDERED" --pid "$SPID"

sleep 1
if [ -f "$DPID" ]; then
    echo "Started: daemon pid $(cat "$DPID"), nats pid $(cat "$SPID" 2>/dev/null || echo '?')"
    echo "Log: $LOG"
    echo "Test: curl -s http://127.0.0.1:8222/varz | head -20"
else
    echo "WARN: pidfile not yet written; check $LOG"
fi
