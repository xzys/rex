#!/bin/bash
# Start Rex safely — stops ES first, launches termsp+rex, restarts ES on exit.
# Designed to be called via SSH: ssh root@brick '/path/to/start-rex.sh'

APP=/userdata/system/apps/terminal
export LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH"
export PATH="$APP:$PATH"
export TERM=xterm-256color
export SHELL=/bin/bash

/etc/init.d/S31emulationstation stop
sleep 1

# Run in background — termsp takes over display, ES restarts when it exits
nohup bash -c '"$0" -s 24 -f "$1" -b "$2" -e "$3"; /etc/init.d/S31emulationstation start' \
    "$APP/termsp" \
    "$APP/fonts/JetBrainsMono-Regular.ttf" \
    "$APP/fonts/JetBrainsMono-Bold.ttf" \
    "$APP/rex" \
    > /tmp/rex-launch.log 2>&1 &

echo "rex started (pid $!)"
