#!/bin/sh
# Show running status, recent stats, and tail of error log.

ROOT="$HOME/iqplus-publisher"
DPID="$ROOT/run/daemon.pid"
PPID_FILE="$ROOT/run/publisher.pid"
LOG="$ROOT/log/iqplus-publisher.log"
ERRLOG="$ROOT/log/iqplus-publisher.error.log"

echo "=== iqplus-publisher status ==="
if [ -f "$DPID" ] && kill -0 "$(cat "$DPID")" 2>/dev/null; then
    DP=$(cat "$DPID")
    PP=$(cat "$PPID_FILE" 2>/dev/null || echo "?")
    echo "RUNNING — daemon pid $DP, publisher pid $PP"
    ps -o pid,etime,rss,vsz,command -p "$DP" "$PP" 2>/dev/null
else
    echo "NOT RUNNING"
fi

echo ""
echo "=== Last 5 stats lines (search 'stats') ==="
grep -i 'stats' "$LOG" 2>/dev/null | tail -5 || echo "(no log yet)"

echo ""
echo "=== Last 10 ERROR/WARN lines ==="
grep -iE '"level":"(error|warn)"|ERROR|WARN' "$LOG" 2>/dev/null | tail -10 || echo "(no errors)"

if [ -f "$ERRLOG" ]; then
    echo ""
    echo "=== Tail of $ERRLOG ==="
    tail -10 "$ERRLOG"
fi
