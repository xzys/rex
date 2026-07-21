#!/bin/bash
# Start Rex — launches termsp with the Debian chroot.
# ES is already disabled via system.es.atstartup=0 in batocera.conf.
# Users launch ES from the games menu tile if desired.
# Designed to be called via SSH: ssh brick-root '/path/to/start-rex.sh'

APP=/userdata/system/apps/rex

# Stage host's brightness binary into the chroot-visible app dir (see Rex.sh
# for context: LCD brightness is driven by /dev/disp ioctl via this binary).
if [ -x /usr/bin/brightness ]; then
    cp -f /usr/bin/brightness "$APP/brightness" && chmod 755 "$APP/brightness"
fi

# Install ConnMan dbus policy if missing — root /etc is on tmpfs overlay so this
# is lost on reboot. Lets uid 1000 (rex inside chroot) talk to net.connman for
# `rex wifi`. No-op if already present or source file missing.
if [ -f "$APP/connman-rex.conf" ] && [ ! -f /etc/dbus-1/system.d/connman-rex.conf ]; then
    cp "$APP/connman-rex.conf" /etc/dbus-1/system.d/connman-rex.conf
    chmod 644 /etc/dbus-1/system.d/connman-rex.conf
    dbus-send --system --type=method_call --dest=org.freedesktop.DBus / \
        org.freedesktop.DBus.ReloadConfig >/dev/null 2>&1 || true
fi

# Snapshot whatever wifi credentials ES has left in batocera.conf into the
# persistent ConnMan provisioning file (/userdata/system/network-connman.config).
# /var/lib/connman is on the volatile rootfs and only the current batocera.conf
# SSID gets re-provisioned at boot; this accumulates every network the user has
# ever connected to via ES so they auto-reconnect across reboots. Must run as
# root because /userdata/system is root-owned. Best-effort.
"$APP/debian-enter.sh" --root /home/rex/bin/rex wifi capture >>/tmp/rex-launch.log 2>&1 || true

# If ES is running (debug mode, launched from tools menu), stop it to free
# the framebuffer before termsp starts. In production mode ES is already
# disabled via atstartup=0, so this is a no-op.
if pidof emulationstation >/dev/null 2>&1; then
    /etc/init.d/S31emulationstation stop >/dev/null 2>&1
fi

# Start Rex — termsp in foreground. On any exit (clean tmux shutdown via
# L1+R1, crash, SIGTERM, stop-rex.sh, or rex shutdown menu), just exit.
nohup bash -c '
    APP="$1"
    ES_FLAG=/tmp/.rex-start-es
    cleanup() {
        if [ -f "$APP/shutdown-host.sh" ] && "$APP/shutdown-host.sh"; then
            echo "=== rex exited at $(date -Iseconds), powering off ==="
            return
        fi
        if [ -f "$ES_FLAG" ]; then
            rm -f "$ES_FLAG"
            echo "=== rex exited at $(date -Iseconds), launching EmulationStation ==="
            emulationstation-standalone
            echo "=== ES exited at $(date -Iseconds), restarting Rex ==="
            exec "$APP/start-rex.sh"
        fi
        echo "=== rex exited at $(date -Iseconds) ==="
    }
    trap cleanup EXIT

    echo ""
    echo "=== rex starting at $(date -Iseconds) ==="
    env \
        LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH" \
        PATH="$APP:$PATH" \
        TERM=xterm-256color \
        SHELL=/bin/bash \
        "$APP/termsp" -c 0 -s 24 \
        -f "$APP/fonts/JetBrainsMono-Regular.ttf" \
        -b "$APP/fonts/JetBrainsMono-Bold.ttf" \
        -e "$APP/debian-rex.sh"
' _ "$APP" >> /tmp/rex-launch.log 2>&1 &

echo "rex started (session pid $!)"
