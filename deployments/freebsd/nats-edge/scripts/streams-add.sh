#!/bin/sh
# Create the four IDX streams on the EDGE nats-server (10.10.8.1:4222).
#
# Run from anywhere with the `nats` CLI installed. Requires:
#   nats context save iqplus-edge \
#     --server nats://10.10.8.1:4222 --token "$NATS_TOKEN"
#   nats context select iqplus-edge
#
# Edge stream policy berbeda dari main:
#   - retention 6h cukup (catchup window utk main mirror)
#   - discard NEW (bukan "old") — kalau penuh, REJECT publisher → backpressure
#     ke TCP, jangan silently drop record lama.
#   - replicas 1 (single node)
#   - dupe-window 5m (lebih panjang dari main, antisipasi main lambat replikasi)

set -e

if ! command -v nats >/dev/null 2>&1; then
    echo "ERROR: nats CLI not found. Install: https://docs.nats.io/using-nats/nats-tools/nats_cli" >&2
    exit 1
fi

echo "Target server:"
nats account info --server nats://10.10.8.1:4222 || true
echo ""

retention_args() {
    cat <<'EOF'
--storage file
--replicas 1
--retention limits
--max-msgs=-1
--max-bytes=-1
--max-msg-size 1MB
--discard new
--dupe-window 5m
--defaults
EOF
}

# IDX_TICK — high-volume tick data, 6h retention
nats stream add IDX_TICK \
  --subjects "idx.trade.>,idx.order.>,idx.tradedone.>,idx.resend.>" \
  --max-age 6h \
  $(retention_args)

# IDX_QUOTE — quote + best quote
nats stream add IDX_QUOTE \
  --subjects "idx.quote.>,idx.bestquote.>" \
  --max-age 6h \
  $(retention_args)

# IDX_META — session/activity/summary/top20/nbs
nats stream add IDX_META \
  --subjects "idx.status.>,idx.activity.>,idx.summary.>,idx.top20.>,idx.nbs.>" \
  --max-age 24h \
  $(retention_args)

# IDX_NEWS — news
nats stream add IDX_NEWS \
  --subjects "idx.news.>" \
  --max-age 24h \
  $(retention_args)

echo ""
echo "Done. Verify:"
echo "  nats stream ls"
echo "  nats stream report"
