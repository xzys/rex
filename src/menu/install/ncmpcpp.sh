#!/bin/bash
# Install ncmpcpp (ncurses MPD client) from Debian apt.
# https://github.com/ncmpcpp/ncmpcpp
set -e

echo "installing ncmpcpp + mpd via apt"
sudo apt-get update
sudo apt-get install -y ncmpcpp mpd

echo "starting mpd (auto-starts on boot via /etc/rc*.d/S*mpd)"
sudo /etc/init.d/mpd start || true

echo
echo "ncmpcpp installed at $(which ncmpcpp)"
echo "mpd installed at $(which mpd)"
pgrep -a mpd || echo "mpd is not running — check /var/log/mpd/mpd.log"
echo "press enter to close..."
read -r _
