#!/bin/sh
# Unmount bind mounts for the Debian chroot. Run after exiting the chroot
# to avoid stale mounts (e.g. before unmounting /userdata or shutting down).

set -u

DEBIAN=/userdata/debian

is_mounted() {
    grep -q " $1 " /proc/mounts
}

for m in dev/pts dev proc sys tmp userdata knulli-lib; do
    if is_mounted "$DEBIAN/$m"; then
        umount "$DEBIAN/$m" || umount -l "$DEBIAN/$m"
    fi
done
