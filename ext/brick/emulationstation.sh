#!/bin/sh
# Called from the rex menu (inside the Debian chroot). Writes a trigger on
# /tmp (bind-mounted from the host, world-writable) so start-rex.sh's EXIT
# trap launches EmulationStation, then kills the tmux server so the session
# unwinds: tmux exits → termsp child exits → termsp exits → EXIT trap fires.
touch /tmp/.rex-start-es
exec tmux kill-server
