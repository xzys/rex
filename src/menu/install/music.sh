#!/bin/bash
# Mount seaking:data/media/music at /var/lib/mpd/music via sshfs (foreground).
# Quitting (Ctrl-C) unmounts. mpd will pick up the music while mounted.
set -e

HOST="100.74.111.31"
REMOTE="pi@$HOST:data/media/music"
MOUNT="/var/lib/mpd/music/remote"
KEY="/home/rex/.ssh/id_ed25519"
KNOWN="/home/rex/.ssh/known_hosts"

# Bail early if seaking isn't reachable — avoids a hang on sshfs connect.
if ! ssh -i "$KEY" -o UserKnownHostsFile="$KNOWN" \
        -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new \
        "pi@$HOST" true 2>/dev/null; then
    echo "seaking ($HOST) is not reachable — skipping mount"
    echo "press enter to close..."
    read -r _
    exit 0
fi

# mpd uid/gid so files appear owned by mpd:audio inside the mount
MPD_UID="$(id -u mpd)"
MPD_GID="$(getent group audio | cut -d: -f3)"

if ! command -v sshfs >/dev/null; then
    echo "sshfs not installed — installing via apt"
    sudo apt-get update
    sudo apt-get install -y sshfs
fi

# Make sure user_allow_other is set so mpd (running as user mpd) can read the mount
if ! grep -q '^user_allow_other' /etc/fuse.conf 2>/dev/null; then
    echo 'user_allow_other' | sudo tee -a /etc/fuse.conf >/dev/null
fi

# If something is already mounted there, unmount it first
if mountpoint -q "$MOUNT"; then
    echo "$MOUNT already mounted — unmounting"
    sudo fusermount -u "$MOUNT" || sudo umount "$MOUNT"
fi

cleanup() {
    echo
    echo "unmounting $MOUNT"
    sudo fusermount -u "$MOUNT" 2>/dev/null || sudo umount "$MOUNT" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "mounting $REMOTE -> $MOUNT (Ctrl-C to unmount and quit)"
sudo sshfs -f "$REMOTE" "$MOUNT" \
    -o IdentityFile="$KEY",UserKnownHostsFile="$KNOWN" \
    -o allow_other,default_permissions,reconnect \
    -o ServerAliveInterval=30,ServerAliveCountMax=3 \
    -o uid="$MPD_UID",gid="$MPD_GID",umask=022 \
    -o StrictHostKeyChecking=accept-new
