#!/bin/bash
# postshare.sh — preseeded on the BATOCERA (FAT32) partition. Runs at
# S12populateshare on every boot. Idempotent.
#
# Two-boot strategy for fresh cards:
#
#   Boot 1: Knulli's autoresize grows SHARE to full card as exFAT, disarms.
#           We detect a non-ext4 /userdata, drop /boot/boot-custom.sh (an S00
#           hook that runs mkfs.ext4 on the unmounted device), and reboot.
#
#   Boot 2: S00bootcustom runs boot-custom.sh before anything mounts SHARE.
#           Device is full-size and unmounted → mkfs.ext4 succeeds cleanly.
#           S11share mounts ext4, S12 populates the skeleton, then we install
#           and start Rex.
#
#   Boot 3+: Already installed (MARKER_DONE exists). Start Rex directly.
#
# ES is disabled via system.es.atstartup=0 in /userdata/system/batocera.conf.
# S31emulationstation reads this and exits without starting, so Rex gets the
# screen on every boot. Users launch ES from the games menu tile if they want.

LOG=/var/log/rex-postshare.log

set -u

ACTION="${1:-start}"
[ "$ACTION" = "start" ] || exit 0

# All output from this point goes to the log and stderr.
exec 1>>"$LOG"
exec 2>&1

MARKER_DONE=/userdata/.rex-installed
BOOT_BUNDLE=/boot/rex-bundle
BUNDLE=/userdata/rex-bundle
PAYLOAD="$BUNDLE/rex-payload.tar.gz"
STAGE=/userdata/rex-install
BOOTCUSTOM=/boot/boot-custom.sh
APP=/userdata/system/apps/rex

# --- Boot 1: not ext4 — stage the S00 conversion and reboot -------------
SHARE_FS="$(awk '$2 == "/userdata" {print $3}' /proc/mounts | tail -n1)"

if [ "$SHARE_FS" != "ext4" ]; then
    echo "postshare: /userdata is ${SHARE_FS:-unmounted}, staging S00 ext4 conversion" >&2

    if [ -f "$BOOTCUSTOM" ]; then
        echo "postshare: $BOOTCUSTOM already exists, not rebooting" >&2
        exit 0
    fi

    mount -o remount,rw /boot 2>/dev/null || true
    cat > "$BOOTCUSTOM" <<'BOOTCUSTOM_EOF'
#!/bin/sh
[ "$1" = "start" ] || exit 0
DEV=/dev/mmcblk0p5
FS="$(blkid -o value -s TYPE "$DEV" 2>/dev/null || true)"
if [ "$FS" != "ext4" ]; then
    mkfs.ext4 -q -F -L SHARE "$DEV" || exit 1
fi
mount -o remount,rw /boot 2>/dev/null || true
rm -f /boot/boot-custom.sh
sync
BOOTCUSTOM_EOF

    if [ ! -s "$BOOTCUSTOM" ]; then
        echo "postshare: failed to write $BOOTCUSTOM, leaving ES up" >&2
        exit 1
    fi
    sync

    echo "postshare: rebooting for ext4 conversion" >&2
    reboot
    exit 0
fi

# --- Boot 2+: ext4 — install (first time) or start Rex ------------------

# Prevent ES from auto-starting and enable WiFi hardware on every boot.
# S31emulationstation reads system.es.atstartup.
# ConnMan reads wifi.enabled to power up the WiFi chip.
mkdir -p /userdata/system
if ! grep -q "^system.es.atstartup=0" /userdata/system/batocera.conf 2>/dev/null; then
    echo "system.es.atstartup=0" >> /userdata/system/batocera.conf
fi
if ! grep -q "^wifi.enabled=1" /userdata/system/batocera.conf 2>/dev/null; then
    echo "wifi.enabled=1" >> /userdata/system/batocera.conf
fi

# Already installed: start Rex and exit.
# Delay start to let the framebuffer initialize (S12 is early; termsp
# needs /dev/fb0 which may not be ready until later in the boot).
if [ -f "$MARKER_DONE" ]; then
    echo "postshare: rex already installed, starting (delayed)" >&2
    if [ -x "$APP/start-rex.sh" ]; then
        (sleep 15 && "$APP/start-rex.sh") &
    fi
    exit 0
fi

# --- First install ------------------------------------------------------
echo "postshare: /userdata is ext4, running first-time install" >&2

# Copy payload from BATOCERA to SHARE.
if [ -d "$BOOT_BUNDLE" ]; then
    mkdir -p "$BUNDLE"
    if [ -f "$BOOT_BUNDLE/rex-payload.tar.gz" ] && [ ! -f "$BUNDLE/rex-payload.tar.gz" ]; then
        cp "$BOOT_BUNDLE/rex-payload.tar.gz" "$BUNDLE/rex-payload.tar.gz"
    fi
fi

[ -f "$PAYLOAD" ] || { echo "postshare: payload not found" >&2; exit 1; }

# Extract and run the installer.
rm -rf "$STAGE"
mkdir -p "$STAGE"
tar xzf "$PAYLOAD" -C "$STAGE" --no-same-owner

INNER="$(find "$STAGE" -maxdepth 1 -mindepth 1 -type d | head -n1)"
[ -d "$INNER" ] || { echo "postshare: no payload dir found" >&2; exit 1; }

echo "$(date -Iseconds): running installer" >&2
if "$INNER/install.sh" 2>&1; then
    touch "$MARKER_DONE"
    echo "$(date -Iseconds): install complete, starting Rex" >&2
else
    echo "$(date -Iseconds): install failed — will retry on next boot" >&2
    exit 1
fi

# Clean up leftover boot-custom.sh from the conversion boot.
rm -f "$BOOTCUSTOM" 2>/dev/null || true

# Start Rex (delayed — framebuffer isn't ready at S12).
if [ -x "$APP/start-rex.sh" ]; then
    (sleep 15 && "$APP/start-rex.sh") &
fi

exit 0
