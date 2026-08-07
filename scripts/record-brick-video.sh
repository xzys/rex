#!/usr/bin/env bash
# Record the Brick's framebuffer to a video.
#
# The asciicast recorder can't be used for EmulationStation: launching ES tears
# down tmux and termsp, which is exactly what that recorder attaches to, and ES
# was never a terminal program. It does draw to /dev/fb0 though (rex doesn't —
# it renders through SDL/MALI), so ffmpeg's fbdev input can capture it.
#
# usage: record-brick-video.sh NAME [SECONDS] [DRIVE_SCRIPT]
#
#   NAME          output name; lands in bin/recordings/NAME.mp4
#   SECONDS       recording length (default 25)
#   DRIVE_SCRIPT  optional local script, run on the Knulli host while
#                 recording. Use scripts/demos/_uinput_key.py to press keys.
#
# Launches ES if it isn't already up, and restores rex afterwards.
set -euo pipefail

NAME=${1:?usage: record-brick-video.sh NAME [SECONDS] [DRIVE_SCRIPT]}
SECS=${2:-25}
DRIVE=${3:-}

# ~30fps is the ceiling: the fbdev read path caps there, so asking for 60 just
# returns the same ~30 (237 frames over 8s). Scaling during capture is what
# makes 30 land almost complete — at full 1024x768 a 30fps request drops to 27.
# 576x432 is also roughly the size the site plays it back at, so this doubles
# as the web encode.
FPS=30
SCALE=576:432
REMOTE=/tmp/rex-video-$NAME.mp4
LOCAL=bin/recordings/$NAME.mp4
APP=/userdata/system/apps/rex

if ! ssh brick-root 'pidof emulationstation >/dev/null'; then
    echo "launching EmulationStation (this stops rex)"
    # Same trigger the games tile uses: flag the EXIT trap, then unwind tmux.
    ssh brick-root "touch /tmp/.rex-start-es" || true
    ssh brick 'tmux kill-server' 2>/dev/null || true
    for _ in $(seq 1 30); do
        ssh brick-root 'pidof emulationstation >/dev/null' && break
        sleep 3
    done
    ssh brick-root 'pidof emulationstation >/dev/null' || { echo "ES did not start" >&2; exit 1; }
    sleep 8   # let the system carousel settle before recording
fi

scp -q "$(dirname "$0")/demos/_uinput_key.py" brick-root:/tmp/rex-uinput-key.py
DRIVE_REMOTE=""
if [ -n "$DRIVE" ]; then
    DRIVE_REMOTE=/tmp/rex-vdrive-$$.sh
    scp -q "$DRIVE" "brick-root:$DRIVE_REMOTE"
fi

echo "recording ${NAME} for ${SECS}s at ${FPS}fps"
# libx264 ultrafast keeps up with 1024x768@15 on the A53s (~0.99x). The
# hardware h264_v4l2m2m encoder is listed but produces no packets.
ssh brick-root "
    ffmpeg -hide_banner -loglevel error -f fbdev -framerate $FPS -i /dev/fb0 \
        -t $SECS -vf scale=$SCALE -c:v libx264 -preset ultrafast -pix_fmt yuv420p \
        -movflags +faststart -y '$REMOTE' &
    FFPID=\$!
    ${DRIVE_REMOTE:+sh $DRIVE_REMOTE || true}
    wait \$FFPID
    ${DRIVE_REMOTE:+rm -f $DRIVE_REMOTE}
"

mkdir -p "$(dirname "$LOCAL")"
scp -q "brick-root:$REMOTE" "$LOCAL"
ssh brick-root "rm -f '$REMOTE'"
echo "saved $LOCAL"

echo "restoring rex"
# Killing ES externally does not reliably trip start-rex.sh's restart chain,
# so start it explicitly rather than waiting on the EXIT trap.
# Bracket the pattern: `pkill -f emulationstation` also matches the remote
# shell running this very command, killing it before its own `|| true` runs.
# ssh then returns 255 and set -e aborts the script — leaving ES dead and rex
# never restarted.
ssh brick-root "pkill -f '[e]mulationstation' || true" || true
sleep 8
# Keep the check and the launch as separate ssh calls. Folded into one as
# `pidof ... || setsid ... &`, the `&` backgrounds the whole expression and ssh
# closes before setsid detaches, leaving the device dark. setsid and </dev/null
# are what let the launcher outlive the ssh session.
if ! ssh brick-root 'pidof termsp >/dev/null'; then
    ssh brick-root "setsid nohup $APP/start-rex.sh >/dev/null 2>&1 </dev/null &" || true
fi
sleep 20
ssh brick-root 'pidof termsp >/dev/null && echo "rex is back" || echo "WARNING: rex did not come back" >&2'
