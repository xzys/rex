#!/bin/bash
# Stop Rex and restart ES.
# Designed to be called via SSH: ssh root@brick '/path/to/stop-rex.sh'

pkill -f termsp
pkill -f "rex server"
sleep 1
/etc/init.d/S31emulationstation start
echo "rex stopped, ES restarting"
