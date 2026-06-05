#!/bin/sh
# Start iqplus-publisher under daemon(8) supervisor.
#
# daemon(8) flags:
#   -r          : restart child if it exits unexpectedly (with backoff)
#   -R 5        : restart delay 5s
#   -P pidfile  : pid of the daemon supervisor itself
#   -p pidfile  : pid of the child (publisher binary)
#   -t title    : process title visible in `ps`
#   -o logfile  : redirect child stdout+stderr to file
#   -f          : redirect descriptors before exec (clean detach)
#
# Stop with: ./stop.sh

set -e

ROOT="$HOME/iqplus-publisher"
BIN="$ROOT/bin/iqplus-publisher"
ENVFILE="$ROOT/bin/iqplus-publisher.env"
LOG="$ROOT/log/iqplus-publisher.log"
DPID="$ROOT/run/daemon.pid"
PPID_FILE="$ROOT/run/publisher.pid"

if [ ! -x "$BIN" ]; then
    echo "ERROR: binary not found or not executable: $BIN" >&2
    exit 1
fi
if [ ! -f "$ENVFILE" ]; then
    echo "ERROR: env file missing: $ENVFILE" >&2
    exit 1
fi

if [ -f "$DPID" ] && kill -0 "$(cat "$DPID")" 2>/dev/null; then
    echo "Already running (daemon pid $(cat "$DPID"))."
    exit 0
fi

# Pre-touch log so tail -f works even before first write.
: >> "$LOG"

/usr/sbin/daemon \
    -r -R 5 \
    -P "$DPID" \
    -p "$PPID_FILE" \
    -t iqplus-publisher \
    -o "$LOG" \
    -f \
    "$BIN"

# daemon detaches and writes pidfile asynchronously.
sleep 1
if [ -f "$DPID" ]; then
    echo "Started: daemon pid $(cat "$DPID"), publisher pid $(cat "$PPID_FILE" 2>/dev/null || echo '?')"
    echo "Log: $LOG"
else
    echo "WARN: pidfile not yet written; check $LOG"
fi
