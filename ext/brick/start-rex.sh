#!/bin/bash
# Start Rex safely — freezes ES, launches termsp+rex.
# Use stop-rex.sh to stop rex and unfreeze ES.
# Designed to be called via SSH: ssh root@brick '/path/to/start-rex.sh'

APP=/userdata/system/apps/rex

# Freeze ES instead of stopping it — avoids disrupting audio/dbus
ES_PID=$(pidof emulationstation)
if [ -n "$ES_PID" ]; then
    kill -STOP $ES_PID
fi

# Run in background — env vars scoped to termsp process only
nohup env \
    LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH" \
    PATH="$APP:$PATH" \
    TERM=xterm-256color \
    SHELL=/bin/bash \
    "$APP/termsp" -s 24 \
    -f "$APP/fonts/JetBrainsMono-Regular.ttf" \
    -b "$APP/fonts/JetBrainsMono-Bold.ttf" \
    -e "$APP/rex" \
    > /tmp/rex-launch.log 2>&1 &

echo "rex started (pid $!)"
