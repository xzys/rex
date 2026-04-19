#!/bin/bash
# Runs inside the Debian chroot as termsp's PTY child. Sets env and execs rex.
# Invoked via: start-rex.sh → termsp -e debian-rex.sh → debian-enter.sh → here.
# termsp itself stays on the Knulli host (it needs Knulli's patched SDL2 +
# PowerVR blobs for display); rex only uses SDL for joystick events, so it
# runs cleanly against Debian's SDL2 inside the chroot.

set -eu

APP=/userdata/system/apps/rex

export LD_LIBRARY_PATH="$APP/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
# Append $APP so Debian's tmux/pkill/pidof win over the cross-compiled
# copies in $APP (those were built against Knulli's libc).
export PATH="$PATH:$APP"
export TERM="${TERM:-xterm-256color}"
export SHELL=/bin/bash
export HOME=/root

cd "$APP"
exec ./rex
