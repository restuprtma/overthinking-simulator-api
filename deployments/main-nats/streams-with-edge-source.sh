#!/bin/sh
# Tambahkan source dari edge ke stream IDX_* yang sudah ada di main.
#
# PENTING: skrip ini melakukan IN-PLACE EDIT (`nats stream edit`), BUKAN
# drop & recreate. Alasan: streams existing sudah punya consumer aktif
# dengan progress state — consumer akan reset/replay kalau stream-nya
# di-drop.
#
# Pre-req:
#   - Leafnode antara main ↔ edge sudah up (lihat docs/infra/iqplus-edge-topology.md).
#   - Verify dulu: nats req '$JS.edge.API.INFO' '' --timeout 3s
#   - Edge sudah punya stream IDX_TICK/QUOTE/META/NEWS kosong.
#
# Idempotent: aman re-run. `nats stream edit` cuma apply diff.

set -e

if ! command -v nats >/dev/null 2>&1; then
    echo "ERROR: nats CLI not found." >&2
    exit 1
fi

# Server URL + token. Bisa override lewat env atau set context.
SERVER="${NATS_URL:-nats://10.10.8.2:4222}"
TOKEN="${NATS_TOKEN:?NATS_TOKEN env required (token main)}"

NATS="nats --server $SERVER --token $TOKEN"

echo "=== Pre-flight ==="
echo "Server: $SERVER"
$NATS account info 2>&1 | head -5 || { echo "ERROR: cannot connect"; exit 1; }
echo ""

# Verify edge JS reachable lewat leaf.
echo "Verify edge JetStream domain reachable..."
if ! $NATS req '$JS.edge.API.INFO' '' --timeout 3s 2>&1 | grep -q '"type"'; then
    cat >&2 <<EOF
ERROR: tidak dapat reach edge JetStream domain.

Cek:
  1. Leafnode sudah up: curl -s http://10.10.8.2:8222/varz | jq .leafnodes
     Harus > 0 setelah edge connect.
  2. Edge nats-server jalan dan domain di config = "edge".
  3. LEAF_TOKEN cocok di kedua sisi.
EOF
    exit 1
fi
echo "OK: edge reachable"
echo ""

# Helper: add --source ke stream existing. NATS CLI tidak punya cara langsung
# edit source via flag — perlu pakai JSON config. Kita ambil config existing,
# patch dengan source array, lalu apply via stream update.

apply_source() {
    name="$1"
    echo "=== $name ==="

    # Cek apakah sudah punya source untuk avoid double-add.
    existing=$($NATS stream info "$name" --json 2>/dev/null | jq -c '.config.sources // []')
    if echo "$existing" | jq -e ".[] | select(.name == \"$name\")" >/dev/null 2>&1; then
        echo "Source untuk '$name' dari edge SUDAH ada. Skip."
        return 0
    fi

    # Ambil config existing, tambahkan source, write balik.
    tmp=$(mktemp)
    trap 'rm -f "$tmp"' RETURN

    $NATS stream info "$name" --json > "$tmp.orig"

    jq --arg n "$name" '
        .config
        | .sources = (.sources // []) + [{
            name: $n,
            external: {
                api: "$JS.edge.API",
                deliver: ""
            }
        }]
    ' "$tmp.orig" > "$tmp.new"

    echo "Updating $name with source from edge..."
    $NATS stream update "$name" --config "$tmp.new" --force

    echo "Verify source aktif:"
    $NATS stream info "$name" 2>&1 | grep -A6 'Sources:' | head -10 || true
    echo ""
}

echo "Streams yang akan di-update:"
echo "  IDX_TICK   ← edge:IDX_TICK"
echo "  IDX_QUOTE  ← edge:IDX_QUOTE"
echo "  IDX_META   ← edge:IDX_META"
echo "  IDX_NEWS   ← edge:IDX_NEWS"
echo ""
echo "Existing config (replicas, retention, dll) tidak diubah —"
echo "cuma menambah 'sources' field yang point ke domain edge."
echo ""
read -p "Lanjut? [y/N] " confirm
case "$confirm" in
    y|Y) ;;
    *) echo "Aborted."; exit 0;;
esac

apply_source IDX_TICK
apply_source IDX_QUOTE
apply_source IDX_META
apply_source IDX_NEWS

echo ""
echo "Done. Final verify:"
$NATS stream report
