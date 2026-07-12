#!/usr/bin/env bash
# build.sh — produce a rex-edition KNULLI SD image for the TrimUI Brick.
#
# Takes an upstream KNULLI Brick .img.gz and drops the rex bundles
# (payload + pre-baked Debian chroot) onto BATOCERA (FAT32) so they
# survive first-boot reformat. Knulli's rootfs and boot config are left
# stock — first boot resizes SHARE and formats it exFAT as usual.
# postshare.sh (S12) detects exFAT, writes /boot/boot-custom.sh (an S00
# hook that runs mkfs.ext4 before anything mounts SHARE), and reboots.
# On boot 2, boot-custom.sh converts exFAT → ext4 on the unmounted
# device (no FUSE conflict, no gdisk corruption), then self-destructs.
# S12 repopulates the skeleton, postshare copies bundles from /boot/,
# on every boot postshare.sh checks the install marker. First time it runs
# install-brick.sh and starts Rex. Subsequent boots start Rex directly.
# ES is disabled via system.es.atstartup=0 — users launch it from the games menu.
# The Debian chroot is extracted to a plain /userdata/debian directory.
#
# Only the FAT32 BATOCERA partition is modified, via hdiutil — no ext4/squashfs
# work, so this runs natively on a macOS host.
#
# Usage:
#   make build-knulli-rex      (calls this)
#
# Env:
#   KNULLI_IMG_URL  override the upstream KNULLI Brick .img.gz URL
#   KNULLI_IMG_SHA  override the expected sha256 (skips download of .sha256)
#   PAYLOAD         override the rex-brick install tarball path
#   CHROOT          override the brick-debian rootfs tarball path
#   REX_DEBUG=1     leave ES enabled for WiFi config + shell access (debug)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DIST_DIR="$REPO_ROOT/dist"
CACHE_DIR="$DIST_DIR/knulli-cache"
OVERLAY="$REPO_ROOT/scripts/knulli/overlay"

# Pinned upstream image. Bump when refreshing against a new KNULLI release.
KNULLI_IMG_URL="${KNULLI_IMG_URL:-https://github.com/knulli-cfw/distribution/releases/download/20250813/knulli-a133-trimui-brick-gladiator-ii-20250812.img.gz}"
KNULLI_IMG_SHA_URL="${KNULLI_IMG_URL}.sha256"

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

# --- 3. mount BATOCERA partition and inject files ---------------------------
# macOS supports FAT32 natively via hdiutil; no Docker needed.
echo "==> mounting BATOCERA partition and injecting files"

# Attach the image — macOS auto-detects and mounts the FAT32 partition
attach_output=$(hdiutil attach -nobrowse -noautoopen "$work_img" 2>&1)
echo "$attach_output"

# Find the BATOCERA mount point. Take everything from "/Volumes/" to end of line
# (mount points can contain spaces, e.g. "/Volumes/BATOCERA 1" when a stale mount
# of the same label already exists) so we never inject into the wrong volume.
boot_mp=$(echo "$attach_output" | sed -nE 's#.*(/Volumes/.+)$#\1#p' | grep BATOCERA | head -1)
[ -n "$boot_mp" ] || boot_mp=$(echo "$attach_output" | sed -nE 's#.*(/Volumes/.+)$#\1#p' | head -1)
echo "BATOCERA mount point: [$boot_mp]"
[ -d "$boot_mp" ] || { echo "error: could not find BATOCERA mount point" >&2; exit 1; }
# Sanity: the mount must contain knulli's stock rootfs squashfs — proves we
# grabbed this image's BATOCERA, not a leftover mount or an unrelated volume.
[ -f "$boot_mp/boot/batocera" ] || { echo "error: $boot_mp is not a knulli BATOCERA partition" >&2; exit 1; }

# Copy overlay scripts
cp "$OVERLAY/postshare.sh" "$boot_mp/postshare.sh"

# Debug mode: REX_DEBUG=1 leaves ES enabled for WiFi config + shell access.
# Rex is installed but does not auto-start; launch from ES menu.
if [ "${REX_DEBUG:-}" = "1" ]; then
    touch "$boot_mp/.rex-debug"
    echo "==> debug mode: ES will auto-start, Rex available from ES menu"
else
    rm -f "$boot_mp/.rex-debug"
fi

# The rootfs (squashfs) and batocera-boot.conf are left 100% stock. Knulli's
# first boot resizes SHARE to fill the card and formats it exFAT, which boots
# reliably. postshare.sh handles the ext4 conversion via a two-boot S00 hook
# (boot-custom.sh runs mkfs.ext4 on the unmounted device before S11share).
# No patching of S02resize, no format-internal, no gdisk — just stock Knulli
# boot sequence with a drop-in S00 script.
# Belt-and-suspenders: ensure no stale persistent-overlay file from an earlier
# build approach lingers (its initramfs restore is what black-screened).
rm -f "$boot_mp/boot/overlay" "$boot_mp/boot/._overlay" 2>/dev/null || true

# Copy bundles
mkdir -p "$boot_mp/rex-bundle"
cp "$PAYLOAD" "$boot_mp/rex-bundle/rex-payload.tar.gz"
cp "$CHROOT" "$boot_mp/rex-bundle/brick-debian.tar.gz"

echo "Files injected. Contents of BATOCERA:"
ls -la "$boot_mp/"
ls -la "$boot_mp/rex-bundle/"

# Detach
hdiutil detach "$boot_mp" -force 2>/dev/null || diskutil unmount force "$boot_mp" 2>/dev/null || true

# --- 4. recompress -----------------------------------------------------------

out="$DIST_DIR/knulli-rex-brick-${VERSION}.img.gz"
echo "==> compressing to $(basename "$out")"
rm -f "$out"
gzip -c "$work_img" > "$out"

rm -f "$work_img"

size=$(du -h "$out" | cut -f1)
echo
echo "==> wrote $out ($size)"
echo "    flash with balenaEtcher / dd to a microSD, insert into Brick"
