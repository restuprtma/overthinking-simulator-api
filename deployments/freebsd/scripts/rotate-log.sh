#!/bin/sh
# Manual logrotate: keep current + 7 daily compressed archives.
# Daemon(8) does NOT reopen log on SIGHUP, so we use copytruncate-style.
# Run via cron daily at 00:05.

set -e
ROOT="$HOME/iqplus-publisher"
LOG="$ROOT/log/iqplus-publisher.log"
DATE=$(date +%Y%m%d)

[ -f "$LOG" ] || exit 0

# Snapshot today's log, then truncate the live file (preserves daemon's fd).
cp "$LOG" "$LOG.$DATE"
: > "$LOG"
gzip -f "$LOG.$DATE"

# Keep 7 newest archives, delete the rest.
ls -1t "$LOG".*.gz 2>/dev/null | tail -n +8 | xargs -r rm -f
