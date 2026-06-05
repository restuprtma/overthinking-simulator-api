#!/bin/sh
# Graceful stop: SIGTERM the daemon supervisor, which forwards to nats-server.
# nats-server flushes JetStream file store before exiting (sync_interval bound).

ROOT="$HOME/nats-edge"
DPID="$ROOT/run/daemon.pid"
SPID="$ROOT/run/nats-server.pid"

if [ ! -f "$DPID" ]; then
    echo "Not running (no $DPID)."
    exit 0
fi

PID=$(cat "$DPID")
if ! kill -0 "$PID" 2>/dev/null; then
    echo "Stale pidfile, removing."
    rm -f "$DPID" "$SPID"
    exit 0
fi

echo "Sending SIGTERM to daemon pid $PID..."
kill -TERM "$PID"

# Wait up to 30s for graceful flush.
for i in $(seq 1 30); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "Stopped cleanly."
        rm -f "$DPID" "$SPID"
        exit 0
    fi
    sleep 1
done

echo "Did not stop in 30s; sending SIGKILL."
kill -KILL "$PID" 2>/dev/null || true
rm -f "$DPID" "$SPID"
