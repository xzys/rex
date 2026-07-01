#!/bin/sh
# Check if rex requested a shutdown (via /userdata/system/.rex-shutdown-request).
# If so, power off the device. Called from start-rex.sh's EXIT trap.
if [ -f /userdata/system/.rex-shutdown-request ]; then
    rm -f /userdata/system/.rex-shutdown-request
    /sbin/poweroff
fi
