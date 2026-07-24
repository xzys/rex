#!/bin/bash
# Install ncmpcpp + mpd on Brick (Debian chroot).
# Creates an ncmpcpp wrapper that auto-starts mpd if not already running.
# https://github.com/ncmpcpp/ncmpcpp
set -e

echo "▸ installing ncmpcpp + mpd + mpc via apt"
sudo apt-get update -qq
sudo apt-get install -y ncmpcpp mpd mpc

echo "▸ writing mpd config (~/.config/mpd/mpd.conf)"
mkdir -p ~/.config/mpd/playlists

cat > ~/.config/mpd/mpd.conf <<'MPDCONF'
# mpd config for TrimUI Brick (ALSA output, music in ~/music)
music_directory     "/home/rex/music"
playlist_directory  "~/.config/mpd/playlists"
db_file             "~/.config/mpd/tag_cache"
log_file            "~/.config/mpd/mpd.log"
pid_file            "~/.config/mpd/pid"
state_file          "~/.config/mpd/state"

audio_output {
    type    "alsa"
    name    "Brick"
    device  "hw:0,0"
}

bind_to_address "any"
port            "6600"
MPDCONF

echo "▸ creating ncmpcpp wrapper (~/bin/ncmpcpp)"
mkdir -p ~/bin

cat > ~/bin/ncmpcpp <<'WRAPPER'
#!/bin/bash
# ncmpcpp wrapper — start mpd if needed, then launch ncmpcpp.
set -e
MPD_CONF="${HOME}/.config/mpd/mpd.conf"

if ! pgrep -x mpd >/dev/null; then
    echo "♫ starting mpd..."
    mpd "$MPD_CONF"
    sleep 0.5
    if pgrep -x mpd >/dev/null; then
        echo "  mpd started"
    else
        echo "  mpd failed to start — check ~/.config/mpd/mpd.log"
        echo "  (is EmulationStation holding the audio device?)"
    fi
fi
exec /usr/bin/ncmpcpp "$@"
WRAPPER

chmod +x ~/bin/ncmpcpp

echo
echo "──────────────────────────────────────────"
echo "ncmpcpp ready!"
echo
echo "ncmpcpp  →  $(which ncmpcpp)"
echo "mpd      →  $(which mpd)"
echo "mpc      →  $(which mpc)"
echo
echo "ncmpcpp now auto-starts mpd on first launch."
echo
echo "Music goes in ~/music/ (writable from rex)."
echo "Run 'mpc update' after adding files to rescan."
echo
echo "Audio conflict: if EmulationStation background music is"
echo "on, mpd may fail to open the audio device. Disable it in"
echo "Batocera Sound Settings → Background Music → Off."
echo "──────────────────────────────────────────"
echo
echo "press enter to close..."
read -r _
