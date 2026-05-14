#!/bin/bash
APP=/userdata/system/apps/rex

# Stage host's brightness binary into the chroot-visible app dir. The rex
# Settings brightness control shells out to this (LCD is driven via /dev/disp
# ioctl, only this binary knows the magic; the /sys/class/led entry is a memo
# and doesn't actually change the panel). chmod 755 because cp inherits a
# restrictive umask on /userdata and the rex user needs +x.
if [ -x /usr/bin/brightness ]; then
    cp -f /usr/bin/brightness "$APP/brightness" && chmod 755 "$APP/brightness"
fi

# Install ConnMan dbus policy if missing — root /etc is on tmpfs overlay so this
# is lost on reboot. Lets uid 1000 (rex inside chroot, same kernel uid as host
# batocera) talk to net.connman for `rex wifi`. No-op if already present or
# source file missing.
if [ -f "$APP/connman-rex.conf" ] && [ ! -f /etc/dbus-1/system.d/connman-rex.conf ]; then
    cp "$APP/connman-rex.conf" /etc/dbus-1/system.d/connman-rex.conf
    chmod 644 /etc/dbus-1/system.d/connman-rex.conf
    dbus-send --system --type=method_call --dest=org.freedesktop.DBus / \
        org.freedesktop.DBus.ReloadConfig >/dev/null 2>&1 || true
fi

/etc/init.d/S31emulationstation stop
sleep 1

env \
    LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH" \
    PATH="$APP:$PATH" \
    TERM=xterm-256color \
    SHELL=/bin/bash \
    "$APP/termsp" -s 24 \
    -f "$APP/fonts/JetBrainsMono-Regular.ttf" \
    -b "$APP/fonts/JetBrainsMono-Bold.ttf" \
    -e "$APP/debian-rex.sh"

/etc/init.d/S31emulationstation start
