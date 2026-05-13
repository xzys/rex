#!/bin/bash
# Install lazygit. Official method from
# https://github.com/jesseduffield/lazygit#installation
set -e

cd "$(mktemp -d)"
LAZYGIT_VERSION=$(curl -s "https://api.github.com/repos/jesseduffield/lazygit/releases/latest" \
    | grep -Po '"tag_name": "v\K[^"]*')
LAZYGIT_ARCH=$(uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/x64/')

echo "installing lazygit ${LAZYGIT_VERSION} for ${LAZYGIT_ARCH}"
curl -fLo lazygit.tar.gz \
    "https://github.com/jesseduffield/lazygit/releases/download/v${LAZYGIT_VERSION}/lazygit_${LAZYGIT_VERSION}_Linux_${LAZYGIT_ARCH}.tar.gz"
tar xf lazygit.tar.gz lazygit
sudo install lazygit -D -t /usr/local/bin/

echo
echo "lazygit installed at $(which lazygit)"
echo "press enter to close..."
read -r _
