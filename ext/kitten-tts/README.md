# KittenTTS for TrimUI Brick

Rex uses the unofficial [`second-state/kitten_tts_rs`](https://github.com/second-state/kitten_tts_rs) binaries for optional local text-to-speech on TrimUI Brick. The official KittenML project currently ships Python wheels, not a standalone Brick-friendly binary.

This directory provides:

- `install-brick.sh` — downloads `kitten-tts-aarch64-linux.tar.gz` and `kitten-tts-models.tar.gz` on the Brick chroot.
- `kitten-speak` — small wrapper that generates a WAV with `kitten-tts` and plays it with `aplay`.

## Install on Brick

From the Rex repo on your workstation:

```bash
make install-kitten-tts-brick
```

That target assumes `ssh brick` connects to the Debian chroot as user `rex`.

Or run manually inside the Brick chroot:

```bash
bash ext/kitten-tts/install-brick.sh
```

## Usage

```bash
kitten-speak --list-voices
kitten-speak "Hello from Rex on the Brick"
kitten-speak --voice Luna --speed 1.2 "Faster Luna voice"
kitten-speak --output /tmp/test.wav --no-play "Generate only"
```

Environment overrides:

- `KITTEN_TTS_BIN` — `kitten-tts` executable.
- `KITTEN_TTS_MODEL` — model directory. Defaults to `~/models/kitten-tts-mini`.
- `KITTEN_TTS_VOICE` — default voice. Defaults to `Bruno`.
- `KITTEN_TTS_SPEED` — default speed. Defaults to `1.0`.
- `KITTEN_TTS_PLAYER` — playback command. Set to `none` to skip playback.
