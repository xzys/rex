#!/bin/sh
# Start OpenSSH inside the Debian chroot (port 2222).
# Call from /userdata/system/custom.sh so it comes up on boot:
#   /userdata/system/apps/rex/debian-sshd.sh
# Connect with: ssh brick  (rex@:2222 via ~/.ssh/config alias)

exec /userdata/system/apps/rex/debian-enter.sh --root '
    mkdir -p /run/sshd
    pgrep -x sshd >/dev/null || /usr/sbin/sshd
'
