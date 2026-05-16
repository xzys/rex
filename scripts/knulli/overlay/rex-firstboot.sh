#!/bin/bash
# rex-firstboot.sh — preseeded on the BATOCERA partition, invoked by the
# auto-generated /userdata/system/custom.sh on every boot.
#
# Two modes:
#   1. First run (no /userdata/.rex-installed): run install-brick.sh.
#      install-brick.sh extracts /userdata/rex-bundle/brick-debian.tar.gz
#      into /userdata/debian, sets up the chroot, and the chroot setup
#      stage runs apt + npm (which DO need network).
#   2. Subsequent runs: exec start-rex.sh.
#
# The chroot tarball lives on the card already (bundled at image build), so
# the chroot extract itself does not need network. The apt/npm part of
# install-brick.sh does — if no network, that stage fails and retries
# next boot. ES is up the whole time so the user can configure wifi via
# the ES menu before retrying.

set -u

STAGE="${1:-}"
MARKER=/userdata/.rex-installed
APP=/userdata/system/apps/rex

# Fast path: already installed → take over the screen.
if [ -f "$MARKER" ] && [ -x "$APP/start-rex.sh" ]; then
    exec "$APP/start-rex.sh"
fi

[ -d "$STAGE" ] || { echo "$(date -Iseconds): no stage dir $STAGE" >&2; exit 1; }

# Wait for network — up to 5 min. install-brick.sh needs net for the apt +
# npm step. User can configure wifi via ES while we wait.
echo "$(date -Iseconds): waiting for network"
for _ in $(seq 1 60); do
    ip route show default 2>/dev/null | grep -q . && break
    sleep 5
done

if ! ip route show default 2>/dev/null | grep -q .; then
    echo "$(date -Iseconds): no network after 5min, giving up — try again next boot"
    exit 1
fi

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
