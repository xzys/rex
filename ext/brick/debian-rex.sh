#!/bin/sh
# Single-binary entrypoint for termsp -e. Enters the Debian chroot and runs rex.
exec /userdata/system/apps/rex/debian-enter.sh /userdata/system/apps/rex/debian-launch-rex.sh
