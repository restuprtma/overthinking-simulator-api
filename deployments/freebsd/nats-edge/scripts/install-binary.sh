#!/bin/sh
# Download official static nats-server binary (FreeBSD/amd64) to ~/nats-edge/bin/.
#
# User-level install — no root, no pkg(8). Versi pinned untuk reproducibility.
#
# Run on the FreeBSD VM (10.10.8.1):
#   sh ~/nats-edge/scripts/install-binary.sh
#
# Re-run-able. Mengambil dari GitHub releases resmi nats-io/nats-server.

set -e

VERSION="${NATS_VERSION:-v2.10.20}"
ROOT="$HOME/nats-edge"
BIN_DIR="$ROOT/bin"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

ARCHIVE="nats-server-${VERSION}-freebsd-amd64.tar.gz"
URL="https://github.com/nats-io/nats-server/releases/download/${VERSION}/${ARCHIVE}"

mkdir -p "$BIN_DIR"

echo "Fetching $URL ..."
fetch -o "$TMP_DIR/$ARCHIVE" "$URL"

echo "Extracting ..."
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

# Archive layout: nats-server-vX.Y.Z-freebsd-amd64/nats-server
SRC_BIN=$(find "$TMP_DIR" -type f -name nats-server -perm +111 | head -1)
if [ -z "$SRC_BIN" ]; then
    echo "ERROR: nats-server binary not found in archive" >&2
    exit 1
fi

install -m 0700 "$SRC_BIN" "$BIN_DIR/nats-server"

echo "Installed: $BIN_DIR/nats-server"
"$BIN_DIR/nats-server" --version
