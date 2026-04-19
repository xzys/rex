#!/bin/bash
# Start Rex safely — freezes ES, launches termsp on Knulli with rex running
# inside the Debian chroot as termsp's PTY child.
# Use stop-rex.sh to stop rex and unfreeze ES.
# Designed to be called via SSH: ssh brick-root '/path/to/start-rex.sh'

APP=/userdata/system/apps/rex

# Freeze ES instead of stopping it — avoids disrupting audio/dbus
ES_PID=$(pidof emulationstation)
if [ -n "$ES_PID" ]; then
    kill -STOP $ES_PID
fi

# termsp runs natively on Knulli (needs Knulli's patched SDL2 + PowerVR blobs);
# its -e child enters the Debian chroot and execs rex there.
nohup env \
    LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH" \
    PATH="$APP:$PATH" \
    TERM=xterm-256color \
    SHELL=/bin/bash \
    "$APP/termsp" -s 24 \
    -f "$APP/fonts/JetBrainsMono-Regular.ttf" \
    -b "$APP/fonts/JetBrainsMono-Bold.ttf" \
    -e "$APP/debian-rex.sh" \
    > /tmp/rex-launch.log 2>&1 &

echo "rex started (pid $!)"
