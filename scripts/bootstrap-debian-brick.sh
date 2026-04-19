#!/usr/bin/env bash
# Bootstrap a Debian arm64 rootfs for TrimUI Brick chroot (Stage 1 of debian-on-brick.md).
# Runs debootstrap inside a linux/arm64 Docker container, then tars up the rootfs.
# Host can be macOS; on Apple Silicon this runs natively without QEMU emulation.

set -euo pipefail

SUITE="${SUITE:-bookworm}"
OUT="${OUT:-bin/brick-debian.tar.gz}"
MIRROR="${MIRROR:-http://deb.debian.org/debian}"

INCLUDES="ca-certificates,curl,wget,git,vim,tmux,htop,openssh-client,build-essential,python3,python3-pip,pkg-config,less,file,procps,libsdl2-2.0-0"

mkdir -p "$(dirname "$OUT")"
OUTDIR="$(cd "$(dirname "$OUT")" && pwd)"
OUTNAME="$(basename "$OUT")"

docker run --rm --platform linux/arm64 \
    -v "$OUTDIR:/output" \
    debian:"$SUITE" bash -eux -c "
apt-get update -qq
apt-get install -y -qq --no-install-recommends debootstrap

debootstrap \
    --arch=arm64 \
    --variant=minbase \
    --include='$INCLUDES' \
    '$SUITE' /rootfs '$MIRROR'

echo 'nameserver 8.8.8.8' > /rootfs/etc/resolv.conf
echo 'brick-debian' > /rootfs/etc/hostname

cat > /rootfs/etc/apt/sources.list <<EOF
deb $MIRROR $SUITE main contrib non-free non-free-firmware
deb $MIRROR $SUITE-updates main contrib non-free non-free-firmware
deb http://security.debian.org/debian-security $SUITE-security main contrib non-free non-free-firmware
EOF

# apt drops to the _apt user for http methods, which fails DNS resolution
# inside a Knulli host chroot. Run acquire methods as root instead.
echo 'APT::Sandbox::User "root";' > /rootfs/etc/apt/apt.conf.d/00no-sandbox

# Clean apt caches to shrink tarball
chroot /rootfs apt-get clean
rm -rf /rootfs/var/lib/apt/lists/*

tar czf /output/$OUTNAME -C /rootfs .
"

echo
echo "Rootfs tarball: $OUT"
ls -lh "$OUT"
