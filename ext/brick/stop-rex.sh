#!/bin/bash
# Stop Rex and restart ES.
# Designed to be called via SSH: ssh root@brick '/path/to/stop-rex.sh'

pkill -f termsp
pkill -f "rex server"
pkill -f "rex keyboard"
tmux kill-session -t rex 2>/dev/null
pkill -f "rex$"
sleep 1
/etc/init.d/S31emulationstation start
echo "rex stopped, ES restarting"
