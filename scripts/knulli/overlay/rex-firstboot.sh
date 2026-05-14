#!/bin/bash
# rex-firstboot.sh — preseeded on the BATOCERA partition, invoked by the
# auto-generated /userdata/system/custom.sh on every boot.
#
# Two modes:
#   1. First run (no /userdata/.rex-installed): wait for network, run the
#      install-brick.sh installer (which fetches the Debian chroot via
#      REX_CHROOT_URL), drop the marker, then fall through to launch rex.
#   2. Subsequent runs: just exec start-rex.sh.
#
# ES is up while this runs, so the user can configure wifi via the ES menu
# before the installer succeeds.

set -u

STAGE="${1:-}"
MARKER=/userdata/.rex-installed
APP=/userdata/system/apps/rex

# Fast path: already installed → take over the screen.
if [ -f "$MARKER" ] && [ -x "$APP/start-rex.sh" ]; then
    exec "$APP/start-rex.sh"
fi

[ -d "$STAGE" ] || { echo "$(date -Iseconds): no stage dir $STAGE" >&2; exit 1; }

# Wait for network — up to 5 min, polling every 5s. User configures wifi via ES
# in the meantime. Skip wait once we see a default route.
echo "$(date -Iseconds): waiting for network"
for _ in $(seq 1 60); do
    ip route show default 2>/dev/null | grep -q . && break
    sleep 5
done

if ! ip route show default 2>/dev/null | grep -q .; then
    echo "$(date -Iseconds): no network after 5min, giving up — try again next boot"
    exit 1
fi

# Run the installer. REX_CHROOT_URL must be baked into this script at image-build
# time (build.sh substitutes it); otherwise install-brick.sh will refuse to
# proceed without an existing /userdata/debian.
export REX_CHROOT_URL="__REX_CHROOT_URL__"

echo "$(date -Iseconds): running installer"
if "$STAGE/install.sh" 2>&1; then
    touch "$MARKER"
    echo "$(date -Iseconds): install complete"
else
    echo "$(date -Iseconds): install failed — will retry on next boot"
    exit 1
fi

# Hand off to start-rex.sh. ES is still up; start-rex stops it.
[ -x "$APP/start-rex.sh" ] && exec "$APP/start-rex.sh"
