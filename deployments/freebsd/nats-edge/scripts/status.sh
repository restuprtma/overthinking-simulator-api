#!/bin/sh
# Show edge nats-server status, JetStream summary, and recent log tail.

ROOT="$HOME/nats-edge"
DPID="$ROOT/run/daemon.pid"
SPID="$ROOT/run/nats-server.pid"
LOG="$ROOT/log/nats-server.log"

echo "=== nats-edge status ==="
if [ -f "$DPID" ] && kill -0 "$(cat "$DPID")" 2>/dev/null; then
    DP=$(cat "$DPID")
    SP=$(cat "$SPID" 2>/dev/null || echo "?")
    echo "RUNNING — daemon pid $DP, nats-server pid $SP"
    ps -o pid,etime,rss,vsz,command -p "$DP" "$SP" 2>/dev/null
else
    echo "NOT RUNNING"
    exit 0
fi

echo ""
echo "=== /varz (server summary) ==="
curl -sS http://127.0.0.1:8222/varz \
    | grep -E '"server_id"|"version"|"connections"|"in_msgs"|"out_msgs"|"in_bytes"|"out_bytes"|"slow_consumers"' \
    || echo "(monitor http port not reachable)"

echo ""
echo "=== /jsz (jetstream summary) ==="
curl -sS 'http://127.0.0.1:8222/jsz?streams=true' \
    | grep -E '"streams"|"messages"|"bytes"|"name"|"consumer_count"' \
    | head -40 \
    || echo "(jetstream not reachable)"

echo ""
echo "=== Disk usage (data/jetstream) ==="
du -sh "$ROOT/data/jetstream" 2>/dev/null || echo "(no data dir yet)"

echo ""
echo "=== Last 5 ERROR/WARN lines ==="
grep -iE '\[ERR\]|\[WRN\]|error|warn' "$LOG" 2>/dev/null | tail -5 || echo "(no log yet)"
