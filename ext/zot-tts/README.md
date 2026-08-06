# zot-tts

A [zot](https://github.com/patriceckhart/zot) extension that speaks the
agent's final answers through on-device neural TTS.
Synthesis runs in-process via sherpa-onnx.

## How it works

- Subscribes (observe-only, never intercepts) to `assistant_message` and
  `turn_end`. It buffers the latest assistant text and speaks it when a turn
  ends with stop reason `end` — a real final answer, not a tool-use step. So
  thinking and tool calls stay on screen and are never read aloud.
- Synthesizes with sherpa-onnx, which keeps the model resident and invokes a
  callback **once per sentence** as that sentence's audio is ready. Samples go
  straight into `aplay` as raw PCM, so playback starts after the first
  sentence rather than the whole answer.
- Speech-friendly phrasing is handled by prompting, not text munging:
  `AGENTS-snippet.md` is appended to `$ZOT_HOME/AGENTS.md` at install so the
  model keeps final answers plain prose.

## Voices

All three run through one code path — sherpa-onnx dispatches on whichever
model sub-config is populated, so switching is a config swap plus a sub-second
model reload, not a different backend.

| Voice | Model | warm RTF on Brick | Rate |
|---|---|---|---|
| `lessac` *(default)* | piper en_US lessac medium | **0.80** | 22050 Hz |
| `ryan` | piper en_US ryan medium | **0.85** | 22050 Hz |
| `kitten` | kitten nano fp16 (8 speakers) | 2.44 | 24000 Hz |

RTF below 1.0 means synthesis outruns playback, so audio plays without gaps.
Kitten is retained as an option but is ~3x slower than realtime and will
stutter on long answers.

Switch at runtime from inside zot — no restart:

```
/voice           # list voices, show the current one
/voice ryan      # switch
```

## Build & install

```bash
make fetch-tts-models      # bin/tts-models/ (~199 MB, all three voices)
make build-zot-tts         # bin/zot-tts/ (aarch64 binary + bundled sherpa libs)
make install-zot-tts-brick # extension + voices + AGENTS.md snippet into the chroot
```

The build is cgo (aarch64 cross-compile in the same Docker image rex uses for
`rex-brick`). It bundles `libsherpa-onnx-c-api.so` and `libonnxruntime.so`
beside the binary and links with `RUNPATH=$ORIGIN`, so the extension resolves
its own libraries and is unaffected by the different onnxruntime that
KittenTTS installs into `/usr/local/lib`.

## Environment

- `ZOT_TTS_VOICE` — starting voice (default `lessac`)
- `ZOT_TTS_MODELS` — models root (default `~/models`), one subdir per voice
- `ZOT_TTS_SPEED` — speed multiplier (default `1.25`)
- `ZOT_TTS_SID` — speaker id, for multi-speaker models like kitten (default 0)
- `ZOT_TTS_THREADS` — inference threads (default 4)
- `ZOT_TTS_PLAYER` — playback command; `{rate}` is substituted with the
  loaded model's sample rate (default `aplay -q -f S16_LE -c 1 -t raw -r {rate}`)

Logs: `$ZOT_HOME/logs/ext-zot-tts.log`.
