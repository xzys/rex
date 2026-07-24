# KittenTTS for TrimUI Brick

On-device text-to-speech via [`second-state/kitten_tts_rs`](https://github.com/second-state/kitten_tts_rs).

The upstream release binaries require glibc ≥ 2.38, but the Brick's Debian Bookworm
chroot has glibc 2.36. We build our own `kitten-tts` binary via Docker cross-compilation
against official onnxruntime v1.23.2 (glibc 2.27 compatible).

## Build (on workstation)

```bash
make build-docker-kitten-tts   # outputs bin/kitten-tts (aarch64, glibc 2.34 max)
```

The Docker build:
- Uses `debian:bookworm-slim` + Rust to target aarch64
- Patches `ort` dependency to disable `api-24` default (onnxruntime 1.23.2 max API is 23)
- Links against official onnxruntime 1.23.2 (`libonnxruntime.so.1` SONAME)

## Install on Brick

The binary + runtime deps are integrated into the standard install path:

1. `scripts/package.sh` bundles `bin/kitten-tts` + official `libonnxruntime.so.1.23.2`
2. `scripts/install/install-brick.sh` copies the binary into the chroot and sets up
   symlinks (`libonnxruntime.so` → `libonnxruntime.so.1` → `.so.1.23.2`)
3. `espeak-ng` is installed via apt during chroot setup

For manual/runtime-only install of deps (models, onnxruntime, espeak-ng):

```bash
make install-kitten-tts-brick   # ssh brick into chroot as rex
```

Or inside the chroot:

```bash
bash ext/kitten-tts/install-brick.sh
```

## Dependencies

| Component | Source | Notes |
|-----------|--------|-------|
| `kitten-tts` binary | `make build-docker-kitten-tts` | Built from `second-state/kitten_tts_rs` v0.2.1 |
| `libonnxruntime.so` | Microsoft v1.23.2 | Bundled in package; glibc 2.27 compatible |
| `espeak-ng` | Debian Bookworm apt | Phoneme generation |
| TTS model | HuggingFace `KittenML/kitten-tts-nano-0.8-int8` | ~27MB; downloaded at install time |

## Usage

```bash
kitten-speak --list-voices
kitten-speak "Hello from Rex on the Brick"
kitten-speak --voice Luna --speed 1.2 "Faster Luna voice"
kitten-speak --output /tmp/test.wav --no-play "Generate only"
```

Environment overrides:

- `KITTEN_TTS_BIN` — `kitten-tts` executable.
- `KITTEN_TTS_MODEL` — model directory. Defaults to `~/models/kitten-tts-nano-int8`.
- `KITTEN_TTS_VOICE` — default voice. Defaults to `Bruno`.
- `KITTEN_TTS_SPEED` — default speed. Defaults to `1.0`.
- `KITTEN_TTS_PLAYER` — playback command. Set to `none` to skip playback.
- `LD_LIBRARY_PATH` — must include `/usr/local/lib` for `libonnxruntime.so.1`.

## Voices

8 voices available (nano model): Bella, Jasper, Luna, Bruno, Rosie, Hugo, Kiki, Leo.
