#!/bin/bash
# Stop Rex and unfreeze ES.
# Designed to be called via SSH: ssh root@brick '/path/to/stop-rex.sh'

# Gracefully stop termsp — SIGTERM triggers SDL cleanup via signal handler
TERMSP_PID=$(pidof termsp)
if [ -n "$TERMSP_PID" ]; then
    kill "$TERMSP_PID"
    for i in $(seq 1 10); do
        kill -0 "$TERMSP_PID" 2>/dev/null || break
        sleep 0.5
    done
    kill -0 "$TERMSP_PID" 2>/dev/null && kill -9 "$TERMSP_PID"
fi

pkill -f "rex server"
pkill -f "rex keyboard"
/userdata/system/apps/rex/tmux kill-session -t rex 2>/dev/null
pkill -f "rex$"
sleep 0.5

# Unfreeze ES — it never stopped, audio/dbus are still intact
ES_PID=$(pidof emulationstation)
if [ -n "$ES_PID" ]; then
    kill -CONT $ES_PID
    echo "rex stopped, ES resumed"
else
    echo "rex stopped (ES was not running)"
fi
