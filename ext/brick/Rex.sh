#!/bin/bash
APP=/userdata/system/apps/terminal
export LD_LIBRARY_PATH="$APP/lib:$LD_LIBRARY_PATH"
export PATH="$APP:$PATH"
export TERM=xterm-256color
export SHELL=/bin/bash

/etc/init.d/S31emulationstation stop
sleep 1

"$APP/termsp" -s 24 \
    -f "$APP/fonts/JetBrainsMono-Regular.ttf" \
    -b "$APP/fonts/JetBrainsMono-Bold.ttf" \
    -e "$APP/rex"

/etc/init.d/S31emulationstation start
