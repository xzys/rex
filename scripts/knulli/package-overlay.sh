#!/usr/bin/env bash
# package-overlay.sh — produce a rex overlay tarball for the TrimUI Brick.
#
# Packages our scripts, payload, and pre-baked Debian chroot into a single
# tar.gz. The user downloads this + the upstream KNULLI image, flashes KNULLI,
# extracts the overlay onto the FAT32 BATOCERA partition, and boots.
# No Knulli redistribution — the user sources Knulli themselves.
#
# Usage:
#   make package-overlay      (calls this)
#
# Env:
#   PAYLOAD   override the rex-brick install tarball path
#   CHROOT    override the brick-debian rootfs tarball path

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST_DIR="$REPO_ROOT/dist"
OVERLAY="$REPO_ROOT/scripts/knulli/overlay"

VERSION="${VERSION:-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)}"
PAYLOAD="${PAYLOAD:-$DIST_DIR/rex-brick-${VERSION}.tar.gz}"
CHROOT="${CHROOT:-$REPO_ROOT/bin/brick-debian.tar.gz}"

if [ ! -f "$PAYLOAD" ]; then
    echo "error: $PAYLOAD missing — run 'make package-brick' first" >&2
    exit 1
fi
if [ ! -f "$CHROOT" ]; then
    echo "error: $CHROOT missing — run 'make bootstrap-debian' first" >&2
    exit 1
fi

echo "==> packaging overlay for $VERSION"

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

# Mirror the layout postshare.sh expects on the FAT32 partition:
#   postshare.sh
#   rex-bundle/
#     rex-payload.tar.gz
#     brick-debian.tar.gz
cp "$OVERLAY/postshare.sh" "$stage/"
mkdir -p "$stage/rex-bundle"
cp "$PAYLOAD" "$stage/rex-bundle/rex-payload.tar.gz"
cp "$CHROOT"   "$stage/rex-bundle/brick-debian.tar.gz"

out="$DIST_DIR/rex-brick-overlay-${VERSION}.tar.gz"
rm -f "$out"
tar czf "$out" -C "$stage" .

size=$(du -h "$out" | cut -f1)
echo "==> wrote $out ($size)"
