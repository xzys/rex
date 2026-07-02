#!/bin/sh
# Check if rex requested a shutdown (via /tmp/.rex-shutdown-request, written
# by shutdown.sh inside the chroot — /tmp is bind-mounted so both sides see
# it). If so, power off. Exits 0 only when powering off so callers can skip
# restarting ES.
if [ -f /tmp/.rex-shutdown-request ]; then
    rm -f /tmp/.rex-shutdown-request
    /sbin/poweroff
    exit 0
fi
exit 1
