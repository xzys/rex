#!/bin/sh
# Enter the Debian chroot on /userdata/debian.
# Safe to run repeatedly; bind mounts are idempotent.
# Usage:
#   debian-enter.sh                  # interactive login shell
#   debian-enter.sh apt update       # run a command inside the chroot

set -eu

DEBIAN=/userdata/debian

if [ ! -d "$DEBIAN" ]; then
    echo "error: $DEBIAN does not exist. Run 'make sync-debian' from your host first." >&2
    exit 1
fi

is_mounted() {
    grep -q " $1 " /proc/mounts
}

is_mounted "$DEBIAN/proc"    || mount -t proc  proc   "$DEBIAN/proc"
is_mounted "$DEBIAN/sys"     || mount -t sysfs sysfs  "$DEBIAN/sys"
is_mounted "$DEBIAN/dev"     || mount --bind   /dev   "$DEBIAN/dev"
is_mounted "$DEBIAN/dev/pts" || mount --bind   /dev/pts "$DEBIAN/dev/pts"
is_mounted "$DEBIAN/tmp"     || mount --bind   /tmp   "$DEBIAN/tmp"

mkdir -p "$DEBIAN/userdata"
is_mounted "$DEBIAN/userdata" || mount --bind /userdata "$DEBIAN/userdata"

# Refresh DNS from the host each entry so WiFi changes propagate.
cp /etc/resolv.conf "$DEBIAN/etc/resolv.conf" 2>/dev/null || true

export HOME=/root
export TERM="${TERM:-xterm-256color}"
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

if [ "$#" -eq 0 ]; then
    exec chroot "$DEBIAN" /bin/bash -l
else
    exec chroot "$DEBIAN" /bin/bash -l -c "$*"
fi
