#!/bin/bash
set -e

TMUX_VERSION=3.3a

cd /tmp
curl -fsSL https://github.com/tmux/tmux/releases/download/${TMUX_VERSION}/tmux-${TMUX_VERSION}.tar.gz -o tmux.tar.gz
tar -xzf tmux.tar.gz
cd tmux-${TMUX_VERSION}

./configure \
    --host=aarch64-linux-gnu \
    --prefix=/usr/local \
    CC=aarch64-linux-gnu-gcc \
    CFLAGS="-I/usr/include/aarch64-linux-gnu" \
    LDFLAGS="-L/usr/lib/aarch64-linux-gnu -static" \
    PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig \
    PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig

make -j$(nproc)
cp tmux /output/tmux
