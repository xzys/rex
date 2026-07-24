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

# Start the WiFi daemon (rex-wifid) — runs as root on the Knulli host, talks
# to ConnMan over D-Bus. Rex inside the chroot talks to it over a Unix socket
# at /tmp/rex-wifi.sock (visible inside the chroot via /tmp bind mount).
# The daemon also snapshots ES wifi credentials on startup.
if ! pidof rex-wifid >/dev/null 2>&1; then
    rm -f /tmp/rex-wifi.sock
    env DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket \
        "$APP/rex-wifid" >>/tmp/rex-launch.log 2>&1 &
    # Wait for the socket to appear (daemon starts quickly).
    for i in $(seq 1 50); do
        [ -S /tmp/rex-wifi.sock ] && break
        sleep 0.1
    done
fi

# Start sshd in the Debian chroot (port 2222) so the user can SSH in
# as rex@brick. /run/sshd is on tmpfs and lost on reboot; create it first.
"$APP/debian-enter.sh" --root "mkdir -p /run/sshd && /usr/sbin/sshd" >>/tmp/rex-launch.log 2>&1 || true

# If ES is running (debug mode, launched from tools menu), stop it to free
# the framebuffer before termsp starts. In production mode ES is already
# disabled via atstartup=0, so this is a no-op.
if pidof emulationstation >/dev/null 2>&1; then
    /etc/init.d/S31emulationstation stop >/dev/null 2>&1
fi

# Start Rex — termsp in foreground. On any exit (clean tmux shutdown via
# L1+R1, crash, SIGTERM, stop-rex.sh, or rex shutdown menu) the EXIT trap
# handles restart/shutdown/ES dispatch.
#
# Guard against double-launch: if termsp is already running, start-rex.sh
# was called while rex is active (e.g. from the EXIT restart loop racing
# with a manual start). Skip silently.
if pidof termsp >/dev/null 2>&1; then
    echo "=== rex already running (termsp pid $(pidof termsp)), skipping start ===" >> /tmp/rex-launch.log
    exit 0
fi

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
        echo "=== rex exited at $(date -Iseconds), restarting Rex ==="
        exec "$APP/start-rex.sh"
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
