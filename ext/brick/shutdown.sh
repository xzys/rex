#!/bin/sh
# Called from the rex menu (inside the Debian chroot). Writes a trigger on
# the shared /userdata bind mount, then kills the tmux server so the whole
# session unwinds: tmux client exits → rex exits → chroot su chain exits →
# termsp exits → the launcher runs shutdown-host.sh, sees the trigger, and
# powers off.
touch /userdata/system/.rex-shutdown-request
exec tmux kill-server
