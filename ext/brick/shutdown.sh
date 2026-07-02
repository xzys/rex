#!/bin/sh
# Called from the rex menu (inside the Debian chroot). Writes a trigger on
# /tmp (bind-mounted from the host, world-writable, self-clearing on reboot —
# /userdata/system is root-owned on ext4 so the rex user can't write there),
# then kills the tmux server so the whole session unwinds: tmux client exits
# → rex exits → chroot su chain exits → termsp exits → the launcher runs
# shutdown-host.sh, sees the trigger, and powers off.
touch /tmp/.rex-shutdown-request
exec tmux kill-server
