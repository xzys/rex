#!/bin/bash
# Stop Rex. The start-rex.sh session's EXIT trap handles restarting ES, so
# killing termsp is enough — its exit unwinds the wrapper shell and the trap
# fires init.d start on ES.
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

# Fallback: if rex was launched the old way (freeze) or no trap fired, make
# sure ES is either running or being brought back.
if ! pidof emulationstation >/dev/null; then
    /etc/init.d/S31emulationstation start >/dev/null 2>&1
fi
ES_PID=$(pidof emulationstation)
if [ -n "$ES_PID" ]; then
    kill -CONT "$ES_PID" 2>/dev/null
fi

echo "rex stopped"
