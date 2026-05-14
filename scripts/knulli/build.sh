#!/usr/bin/env bash
# build.sh — produce a rex-edition KNULLI SD image for the TrimUI Brick.
#
# Takes an upstream KNULLI Brick .img.gz, mounts its BATOCERA (FAT32) partition,
# drops in:
#   - rex-payload.tar.gz (the existing scripts/package.sh brick output)
#   - postshare.sh        (BATOCERA's /boot/postshare.sh hook; runs once)
#   - rex-firstboot.sh    (invoked from generated /userdata/system/custom.sh)
# then repackages.
#
# postshare.sh extracts the payload into /userdata/rex-install/ on first boot
# and wires /userdata/system/custom.sh -> /boot/rex-firstboot.sh. ES still
# boots normally so the user can configure wifi; once network is up,
# rex-firstboot.sh runs install-brick.sh (which fetches the chroot from
# REX_CHROOT_URL), marks installed, and execs start-rex.sh. Subsequent boots
# go straight to rex.
#
# Runs on macOS — uses hdiutil. SHARE is ext4 initially and gets reformatted
# on first boot anyway, so we don't touch it; BATOCERA is FAT32 and stable.
#
# Usage:
#   REX_CHROOT_URL=https://... scripts/knulli/build.sh
#
# Env:
#   REX_CHROOT_URL  required; baked into rex-firstboot.sh so the device can
#                   fetch the Debian arm64 rootfs on first boot
#   KNULLI_IMG_URL  override the upstream KNULLI Brick .img.gz URL
#   KNULLI_IMG_SHA  override the expected sha256
#   PAYLOAD         override the rex-brick install tarball path

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST_DIR="$REPO_ROOT/dist"
CACHE_DIR="$DIST_DIR/knulli-cache"
OVERLAY="$REPO_ROOT/scripts/knulli/overlay"

# Pinned upstream image. Bump when refreshing against a new KNULLI release.
KNULLI_IMG_URL="${KNULLI_IMG_URL:-https://github.com/knulli-cfw/distribution/releases/download/20250813/knulli-a133-trimui-brick-gladiator-ii-20250812.img.gz}"
KNULLI_IMG_SHA_URL="${KNULLI_IMG_URL}.sha256"

REX_CHROOT_URL="${REX_CHROOT_URL:-}"
if [ -z "$REX_CHROOT_URL" ]; then
    echo "error: REX_CHROOT_URL is required" >&2
    echo "  point it at a Debian arm64 rootfs tarball (see 'make bootstrap-debian')" >&2
    exit 1
fi

VERSION="${VERSION:-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)}"
PAYLOAD="${PAYLOAD:-$DIST_DIR/rex-brick-${VERSION}.tar.gz}"

if [ ! -f "$PAYLOAD" ]; then
    echo "error: $PAYLOAD missing — run 'make package-brick' first" >&2
    exit 1
fi

mkdir -p "$CACHE_DIR"

# --- 1. download + verify upstream image ------------------------------------
img_gz_name="$(basename "$KNULLI_IMG_URL")"
img_gz="$CACHE_DIR/$img_gz_name"
sha_file="$img_gz.sha256"

if [ ! -f "$img_gz" ]; then
    echo "==> downloading $img_gz_name"
    curl -fL --progress-bar -o "$img_gz" "$KNULLI_IMG_URL"
fi

if [ ! -f "$sha_file" ]; then
    curl -fsSL -o "$sha_file" "$KNULLI_IMG_SHA_URL"
fi

echo "==> verifying sha256"
expected="$(awk '{print $1}' "$sha_file")"
actual="$(shasum -a 256 "$img_gz" | awk '{print $1}')"
if [ "$expected" != "$actual" ]; then
    echo "error: sha256 mismatch on $img_gz" >&2
    echo "       expected $expected" >&2
    echo "       got      $actual" >&2
    exit 1
fi

# --- 2. decompress into a working copy --------------------------------------
work_img="$CACHE_DIR/work-${VERSION}.img"
echo "==> decompressing to $(basename "$work_img")"
rm -f "$work_img"
gunzip -c "$img_gz" > "$work_img"

# --- 3. attach + locate BATOCERA mount --------------------------------------
echo "==> attaching image"
attach_out="$(hdiutil attach -nobrowse -noverify "$work_img")"
echo "$attach_out"

mount_point="$(printf '%s\n' "$attach_out" | awk -F'\t' '/\/Volumes\/BATOCERA/ {print $NF}' | head -n1)"
disk_id="$(printf '%s\n' "$attach_out" | awk '/\/dev\/disk[0-9]+ / {print $1; exit}')"

if [ -z "$mount_point" ] || [ -z "$disk_id" ]; then
    echo "error: could not find BATOCERA mount in hdiutil output" >&2
    hdiutil detach "$disk_id" 2>/dev/null || true
    exit 1
fi

cleanup() {
    hdiutil detach "$disk_id" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> BATOCERA mounted at $mount_point (disk: $disk_id)"

# --- 4. inject overlay ------------------------------------------------------
echo "==> injecting rex payload"
cp "$PAYLOAD" "$mount_point/rex-payload.tar.gz"
cp "$OVERLAY/postshare.sh" "$mount_point/postshare.sh"

# Substitute REX_CHROOT_URL into rex-firstboot.sh
sed "s|__REX_CHROOT_URL__|$REX_CHROOT_URL|" "$OVERLAY/rex-firstboot.sh" \
    > "$mount_point/rex-firstboot.sh"

# FAT32 doesn't carry the exec bit — that's fine, postshare.sh is invoked by
# Batocera's init via /bin/sh, and rex-firstboot.sh is invoked via custom.sh
# which we'll make exec on the userdata (ext/exFAT) side.

# Flush to disk before detach.
sync

# --- 5. detach + recompress -------------------------------------------------
echo "==> detaching"
hdiutil detach "$disk_id"
trap - EXIT

out="$DIST_DIR/knulli-rex-brick-${VERSION}.img.gz"
echo "==> compressing to $(basename "$out")"
rm -f "$out"
gzip -c "$work_img" > "$out"

# Working .img is large — leave it cached for incremental rebuilds? No, it's
# 4GB+. Delete it.
rm -f "$work_img"

size=$(du -h "$out" | cut -f1)
echo
echo "==> wrote $out ($size)"
echo "    flash with balenaEtcher / dd to a microSD, insert into Brick"
