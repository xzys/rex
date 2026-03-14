#!/bin/bash
set -e

export CC=aarch64-linux-gnu-gcc
export PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig

SDL_CFLAGS=$(pkg-config --cflags sdl2 SDL2_image SDL2_ttf)
SDL_LIBS=$(pkg-config --libs sdl2 SDL2_image SDL2_ttf)

mkdir -p build

$CC -Wall -O3 -flto \
    $SDL_CFLAGS \
    -ITermSP/includes/ \
    '-DSCREENRC="/userdata/system/.screenrc"' \
    TermSP/src/*.c \
    -o build/TermSP \
    -Wl,--gc-sections $SDL_LIBS -lutil -LTermSP/libs -lsdlfox -lvterm \
    -Wl,--allow-shlib-undefined

cp build/TermSP /output/termsp
