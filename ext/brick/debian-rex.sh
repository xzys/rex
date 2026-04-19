#!/bin/sh
# Single-binary entrypoint for termsp -e. Enters the Debian chroot and runs rex
# as user 'rex' (standard Debian home-bin path).
exec /userdata/system/apps/rex/debian-enter.sh /home/rex/bin/rex
