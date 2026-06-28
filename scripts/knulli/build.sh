#!/usr/bin/env bash
# build.sh — produce a rex-edition KNULLI SD image for the TrimUI Brick.
#
# Takes an upstream KNULLI Brick .img.gz, grows the SHARE partition (ext4)
# to fit the rex payload + Debian chroot, drops bundles onto BATOCERA
# (FAT32) so they survive first-boot reformat, and writes the overlay
# scripts alongside them. On first boot, postshare.sh copies bundles from
# /boot/ to /userdata/ (the now-empty exFAT SHARE), extracts the payload,
# and wires up rex-firstboot.sh, which runs install-brick.sh. No network
# needed at first boot — the chroot is pre-baked and already on the card.
#
# Runs the partition-level work inside a privileged Linux Docker container
# so this works from a macOS host (no native ext4 mount).
#
# Usage:
#   make build-knulli-rex      (calls this)
#
# Env:
#   KNULLI_IMG_URL  override the upstream KNULLI Brick .img.gz URL
#   KNULLI_IMG_SHA  override the expected sha256 (skips download of .sha256)
#   PAYLOAD         override the rex-brick install tarball path
#   CHROOT          override the brick-debian rootfs tarball path

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

# --- 3. hand to Docker for partition + filesystem work ----------------------
# Size budget: chroot tarball + payload + slack for extraction + user data.
# Add 1.5GB to be safe. The bundles go on BATOCERA (FAT32, 5GB), but the
# SHARE partition (ext4 → exFAT on first boot) still needs room for the
# extracted chroot (~3GB uncompressed).
echo "==> growing image + bundling chroot/payload via Docker"

stage="$(mktemp -d)"
cp "$PAYLOAD" "$stage/rex-payload.tar.gz"
cp "$CHROOT" "$stage/brick-debian.tar.gz"
cp "$OVERLAY/postshare.sh" "$stage/postshare.sh"
cp "$OVERLAY/rex-firstboot.sh" "$stage/rex-firstboot.sh"
cp "$work_img" "$stage/work.img"

cleanup() { rm -rf "$stage"; }
trap cleanup EXIT

docker run --rm --privileged \
    -v "$stage:/work" \
    debian:bookworm-slim bash -eux <<'DOCKER_EOF'
apt-get update -qq
apt-get install -y -qq --no-install-recommends parted e2fsprogs dosfstools util-linux

cd /work

# Grow .img by 1.5GB
truncate -s +1536M work.img

# Grow last partition to fill the image
parted -s work.img resizepart 2 100%

# Loopback with partition scan
loop=$(losetup --show -fP work.img)
trap "losetup -d $loop" EXIT

# Check + resize ext4 partition 2 (SHARE / userdata)
e2fsck -fy "${loop}p2"
resize2fs "${loop}p2"

# Mount both partitions
mkdir -p /mnt/boot /mnt/share
mount -t vfat "${loop}p1" /mnt/boot
mount -t ext4 "${loop}p2" /mnt/share

# Overlay scripts onto BATOCERA (FAT32)
cp postshare.sh rex-firstboot.sh /mnt/boot/

# Bundle onto BATOCERA (FAT32) — survives first-boot reformat of SHARE.
# Knulli's S02resize unconditionally reformats the SHARE partition as exFAT
# on first boot, destroying anything placed there. BATOCERA is untouched.
mkdir -p /mnt/boot/rex-bundle
cp rex-payload.tar.gz brick-debian.tar.gz /mnt/boot/rex-bundle/

sync
umount /mnt/share /mnt/boot
DOCKER_EOF

# --- 4. recompress + cleanup ------------------------------------------------
cp "$stage/work.img" "$work_img"

out="$DIST_DIR/knulli-rex-brick-${VERSION}.img.gz"
echo "==> compressing to $(basename "$out")"
rm -f "$out"
gzip -c "$work_img" > "$out"

rm -f "$work_img"

size=$(du -h "$out" | cut -f1)
echo
echo "==> wrote $out ($size)"
echo "    flash with balenaEtcher / dd to a microSD, insert into Brick"
