#!/bin/bash
# rex-firstboot.sh — preseeded on the BATOCERA partition, invoked by the
# auto-generated /userdata/system/custom.sh on every boot.
#
# Two modes:
#   1. First run (no /userdata/.rex-installed): run install-brick.sh.
#      install-brick.sh extracts /userdata/rex-bundle/brick-debian.tar.gz
#      into /userdata/debian. The chroot is pre-baked (all apt packages,
#      npm global installs, rex user, locale, sshd config) — no network
#      needed during install.
#   2. Subsequent runs: exec start-rex.sh.
#
# The chroot tarball lives on the card already (copied from BATOCERA to
# /userdata by postshare.sh), so the install is a local extract only.

set -u

STAGE="${1:-}"
MARKER=/userdata/.rex-installed
APP=/userdata/system/apps/rex

# Fast path: already installed → take over the screen.
if [ -f "$MARKER" ] && [ -x "$APP/start-rex.sh" ]; then
    exec "$APP/start-rex.sh"
fi

[ -d "$STAGE" ] || { echo "$(date -Iseconds): no stage dir $STAGE" >&2; exit 1; }

echo "$(date -Iseconds): running installer (pre-baked chroot, no network needed)"
if "$STAGE/install.sh" 2>&1; then
    touch "$MARKER"
    echo "$(date -Iseconds): install complete"
else
    echo "$(date -Iseconds): install failed — will retry on next boot"
    exit 1
fi

# Hand off to start-rex.sh. ES is still up; start-rex stops it.
[ -x "$APP/start-rex.sh" ] && exec "$APP/start-rex.sh"
