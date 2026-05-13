#!/bin/bash
# Start Rex safely — stops ES (reclaims ~90 MB RAM), launches termsp on Knulli
# with rex running inside the Debian chroot as termsp's PTY child.
# Symmetric with Rex.sh (the ES menu launcher). Use stop-rex.sh to stop rex
# early and let ES come back up.
# Designed to be called via SSH: ssh brick-root '/path/to/start-rex.sh'

APP=/userdata/system/apps/rex

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

# Stop ES (reclaims RAM). init.d is the same path Rex.sh uses from the ES menu.
/etc/init.d/S31emulationstation stop >/dev/null 2>&1
sleep 1

# Background a subshell that owns the full lifecycle: launch termsp in the
# foreground, and on ANY exit (clean tmux shutdown via L1+R1, crash, SIGTERM,
# stop-rex.sh) fire an EXIT trap that restarts ES. Symmetric stop/start.
nohup bash -c '
    APP="$1"
    cleanup() {
        /etc/init.d/S31emulationstation start >/dev/null 2>&1
        echo "=== rex exited at $(date -Iseconds), ES restarted ==="
    }
    trap cleanup EXIT

    echo ""
    echo "=== rex starting at $(date -Iseconds) ==="
    # No "exec" here — we need the shell to stay alive past termsp so the
    # EXIT trap fires. env is a single-fork cost, termsp is the long-lived one.
    env \
        LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH" \
        PATH="$APP:$PATH" \
        TERM=xterm-256color \
        SHELL=/bin/bash \
        "$APP/termsp" -s 24 \
        -f "$APP/fonts/JetBrainsMono-Regular.ttf" \
        -b "$APP/fonts/JetBrainsMono-Bold.ttf" \
        -e "$APP/debian-rex.sh"
' _ "$APP" >> /tmp/rex-launch.log 2>&1 &

echo "rex started (session pid $!)"
