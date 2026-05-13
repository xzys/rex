#!/bin/bash
# Install lf (terminal file manager) from the upstream GitHub release.
# https://github.com/gokcehan/lf
set -e

cd "$(mktemp -d)"
case "$(uname -m)" in
    aarch64|arm64) LF_ARCH=arm64 ;;
    x86_64)        LF_ARCH=amd64 ;;
    *) echo "unsupported arch $(uname -m)"; exit 1 ;;
esac

echo "installing lf for linux-${LF_ARCH}"
curl -fLo lf.tar.gz \
    "https://github.com/gokcehan/lf/releases/latest/download/lf-linux-${LF_ARCH}.tar.gz"
tar xf lf.tar.gz lf
sudo install lf -D -t /usr/local/bin/

echo
echo "lf installed at $(which lf)"
echo "press enter to close..."
read -r _
