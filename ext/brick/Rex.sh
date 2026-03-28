#!/bin/bash
APP=/userdata/system/apps/rex

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
    -e "$APP/rex"

/etc/init.d/S31emulationstation start
