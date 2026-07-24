#!/bin/bash
# Install bottom (btm) — system monitor, like htop but prettier.
# https://github.com/ClementTsang/bottom
set -e

BOTTOM_VERSION="0.12.3"
ARCH="$(uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')"
DEB="bottom_${BOTTOM_VERSION}-1_${ARCH}.deb"

echo "▸ installing bottom ${BOTTOM_VERSION} (${ARCH})"
cd "$(mktemp -d)"
curl -fsSLO "https://github.com/ClementTsang/bottom/releases/download/${BOTTOM_VERSION}/${DEB}"
sudo dpkg -i "$DEB"
rm "$DEB"

echo
echo "──────────────────────────────────────────"
echo "btm installed at $(which btm)"
echo "──────────────────────────────────────────"
echo
echo "press enter to close..."
read -r _
