#!/bin/sh
# Enter the Debian chroot on /userdata/debian as user 'rex' (sudo NOPASSWD).
# Safe to run repeatedly; bind mounts are idempotent.
# Usage:
#   debian-enter.sh                  # interactive login shell as rex
#   debian-enter.sh apt update       # run a command inside the chroot as rex
#   debian-enter.sh --root [cmd...]  # enter/run as root (for bootstrap, admin)

set -eu

DEBIAN=/userdata/debian
USER_NAME=rex

if [ "${1:-}" = "--root" ]; then
    USER_NAME=root
    shift
fi

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

export TERM="${TERM:-xterm-256color}"

# `su -` (login shell) sets HOME, PATH, and cd's to the target user's home —
# which also dodges the post-chroot invalid-CWD issue.
if [ "$#" -eq 0 ]; then
    exec chroot "$DEBIAN" su - "$USER_NAME"
else
    exec chroot "$DEBIAN" su - "$USER_NAME" -c "$*"
fi
