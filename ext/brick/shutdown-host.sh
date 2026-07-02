#!/bin/sh
# Check if rex requested a shutdown (via /userdata/system/.rex-shutdown-request).
# If so, power off. Exits 0 only when powering off so callers can skip
# restarting ES.
if [ -f /userdata/system/.rex-shutdown-request ]; then
    rm -f /userdata/system/.rex-shutdown-request
    /sbin/poweroff
    exit 0
fi
exit 1
