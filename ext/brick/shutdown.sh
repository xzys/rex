#!/bin/sh
# Called from the rex menu (inside the Debian chroot). Writes a trigger
# on the shared /userdata bind mount, then exits. The host-side wrapper
# (shutdown-host.sh) picks it up and powers off.
touch /userdata/system/.rex-shutdown-request
exit 0
