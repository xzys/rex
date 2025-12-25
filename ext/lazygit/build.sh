#!/bin/bash

set -e

# install lazygit
go install -ldflags="-s -w" github.com/jesseduffield/lazygit@latest
cp /root/go/bin/linux_arm64/lazygit /output/lazygit

# install git version 2.32.0
GIT_VERSION=2.32.0
cd /tmp
curl -fsSL https://github.com/git/git/archive/refs/tags/v${GIT_VERSION}.tar.gz -o git.tar.gz
tar -xzf git.tar.gz
cd git-${GIT_VERSION}

make configure
CC=aarch64-linux-gnu-gcc \
AR=aarch64-linux-gnu-ar \
PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig \
LDFLAGS="-L/usr/lib/aarch64-linux-gnu" \
CPPFLAGS="-I/usr/include/aarch64-linux-gnu" \
./configure --prefix=/usr/local --host=aarch64-linux-gnu \
    NO_TCLTK=YesPlease \
    NO_GETTEXT=YesPlease \
    NO_PYTHON=YesPlease \
    NO_PERL=YesPlease

make NO_TCLTK=YesPlease NO_GETTEXT=YesPlease NO_PYTHON=YesPlease NO_PERL=YesPlease \
    NO_INSTALL_HARDLINKS=YesPlease

make install NO_TCLTK=YesPlease NO_GETTEXT=YesPlease NO_PYTHON=YesPlease NO_PERL=YesPlease \
    DESTDIR=/tmp/git-install
cp /tmp/git-install/usr/local/bin/git /output/git

