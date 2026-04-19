#!/bin/sh
# Single-binary entrypoint for termsp -e. Enters the Debian chroot and runs rex.
# rex lives inside the chroot at /root/bin/rex (standard Debian home-bin path)
# and uses Debian's apt-installed tmux.
exec /userdata/system/apps/rex/debian-enter.sh /root/bin/rex
