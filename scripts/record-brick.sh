#!/usr/bin/env bash
# Record the Brick's live screen to an asciicast.
#
# rex runs its UI inside a tmux session on the device, so a read-only attach in
# a PTY sized to match that session mirrors the screen exactly. Nothing is
# restarted and the running session is not disturbed — the cast is literally
# what is on the handheld.
#
# usage: record-brick.sh NAME [SECONDS] [DRIVE_SCRIPT]
#
#   NAME          output name; lands in bin/recordings/NAME.cast
#   SECONDS       recording length (default 20)
#   DRIVE_SCRIPT  optional local file of shell commands, run on the device
#                 while recording, to drive the demo with `tmux send-keys`.
#                 Omit it to drive the device by hand with the buttons.
set -euo pipefail

NAME=${1:?usage: record-brick.sh NAME [SECONDS] [DRIVE_SCRIPT]}
SECS=${2:-20}
DRIVE=${3:-}

SESSION=rex
REMOTE=/userdata/recordings/$NAME.cast
LOCAL=bin/recordings/$NAME.cast

# asciinema is a development tool, deliberately kept out of the chroot
# provisioning in `make setup-debian` so it never ships in a release image.
# Install it on demand instead.
if ! ssh brick 'command -v asciinema' >/dev/null 2>&1; then
    echo "installing asciinema into the chroot (dev-only, not part of a release)"
    if ! out=$(ssh brick-root '/userdata/system/apps/rex/debian-enter.sh --root \
        "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq asciinema"' 2>&1); then
        echo "$out" >&2
        echo "failed to install asciinema" >&2
        exit 1
    fi
fi

# Match the recording PTY to the session. tmux sizes a session to its smallest
# attached client, so a mismatch here would visibly resize the handheld.
SIZE=$(ssh brick "tmux display -p -t $SESSION '#{window_width} #{window_height}'")
COLS=${SIZE% *}
ROWS=${SIZE#* }
echo "recording ${NAME} for ${SECS}s at ${COLS}x${ROWS}"

DRIVE_REMOTE=""
if [ -n "$DRIVE" ]; then
    DRIVE_REMOTE=/tmp/rex-drive-$$.sh
    scp -q "$DRIVE" "brick:$DRIVE_REMOTE"
fi

ssh -tt brick "
    export TERM=xterm-256color
    stty rows $ROWS cols $COLS
    ${DRIVE_REMOTE:+sh $DRIVE_REMOTE &}
    asciinema rec --overwrite -q \
        -c 'timeout $SECS tmux attach -t $SESSION -r' '$REMOTE'
    ${DRIVE_REMOTE:+rm -f $DRIVE_REMOTE}
" >/dev/null

mkdir -p "$(dirname "$LOCAL")"
scp -q "brick:$REMOTE" "$LOCAL"
echo "saved $LOCAL"
echo "play: asciinema play $LOCAL"
