#!/bin/sh
# Single-binary entrypoint for termsp -e. Enters the Debian chroot and runs rex
# as user 'rex' (standard Debian home-bin path). Redirect stderr to a persistent
# file so Go runtime fatals and panic stack traces survive crashes — termsp
# assigns its -e child a PTY, so unredirected stderr vanishes into the visible
# terminal rather than reaching termsp's own stderr (rex-launch.log).
echo "" >> /tmp/rex-stderr.log
echo "=== rex starting at $(date -Iseconds) ===" >> /tmp/rex-stderr.log
exec /userdata/system/apps/rex/debian-enter.sh /home/rex/bin/rex 2>>/tmp/rex-stderr.log
