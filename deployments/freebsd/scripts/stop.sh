#!/bin/sh
# Graceful stop: SIGTERM the daemon supervisor, which forwards to the child.
# Publisher main() handles SIGTERM and drains NATS for up to NATS_DRAIN_TIMEOUT.

ROOT="$HOME/iqplus-publisher"
DPID="$ROOT/run/daemon.pid"
PPID_FILE="$ROOT/run/publisher.pid"

if [ ! -f "$DPID" ]; then
    echo "Not running (no $DPID)."
    exit 0
fi

PID=$(cat "$DPID")
if ! kill -0 "$PID" 2>/dev/null; then
    echo "Stale pidfile, removing."
    rm -f "$DPID" "$PPID_FILE"
    exit 0
fi

echo "Sending SIGTERM to daemon pid $PID..."
kill -TERM "$PID"

# Wait up to 35s (publisher drain default 30s + buffer)
for i in $(seq 1 35); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "Stopped cleanly."
        rm -f "$DPID" "$PPID_FILE"
        exit 0
    fi
    sleep 1
done

echo "Did not stop in 35s; sending SIGKILL."
kill -KILL "$PID" 2>/dev/null || true
rm -f "$DPID" "$PPID_FILE"
