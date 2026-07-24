#!/bin/bash
# Stop Rex. Kills termsp and all Rex processes.
# Designed to be called via SSH: ssh brick-root '/path/to/stop-rex.sh'

# Gracefully stop termsp — SIGTERM triggers SDL cleanup via signal handler.
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
pkill -f "tmux .*-s rex" 2>/dev/null
pkill -f "rex$"
pkill -x rex-wifid 2>/dev/null || true
rm -f /tmp/rex-wifi.sock

echo "rex stopped"
