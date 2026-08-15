# rex-tts

On-device neural TTS that speaks a coding agent's final answers. One Go
binary carries the synthesis core (sherpa-onnx, in-process) and two thin
frontends:

- **zot**: the binary is a zot extension (default mode)
- **pi**: `pi-tts.ts` spawns `rex-tts --serve` as a long-lived child and
  forwards answers over JSON lines on stdio

## How it works

- Both frontends observe, never intercept: every assistant message is spoken
  as it completes — interim narration between tool calls as well as the final
  answer. Thinking is never read aloud; it is excluded by block-type
  filtering (pi-tts extracts only `type: "text"` blocks, and zot's host
  builds the event text from `TextBlock` content only). Utterances queue
  (capacity 8) and play back-to-back; when the queue is full the newest
  utterance is dropped rather than stalling the agent.
- Synthesis runs via sherpa-onnx, which keeps the model resident and invokes
  a callback **once per sentence** as that sentence's audio is ready. Samples
  go straight into `aplay` as raw PCM, so playback starts after the first
  sentence rather than the whole answer.
- Model loading is off the startup path. The process answers its frontend's
  handshake (zot's hello, or the first `--serve` request) before the voice is
  loaded, and loads it on a background goroutine. This matters for zot, which
  blocks its own boot for up to 3s waiting on every extension's `ready` frame
  — long enough that a synchronous load made every launch feel stuck.
  Utterances spoken during that window wait in the queue instead of being
  dropped, and `/voice` reports `still loading` until the model is up.
- Speech-friendly phrasing is handled by prompting, not text munging. For
  zot, `AGENTS-snippet.md` is appended to `$ZOT_HOME/AGENTS.md` at install;
  pi-tts injects the same text into the system prompt on every run.

## Voices

Every voice is a single name. They run through one code path — sherpa-onnx
dispatches on whichever model sub-config is populated, so switching is a
config swap plus a sub-second model reload, not a different backend.

| Voice | Model | warm RTF on Brick | Rate |
|---|---|---|---|
| `lessac` *(default)* | piper en_US lessac medium | **0.80** | 22050 Hz |
| `ryan` | piper en_US ryan medium | **0.85** | 22050 Hz |
| `bella` `jasper` `luna` `bruno` `rosie` `hugo` `kiki` `leo` | kitten nano fp16 | 2.44 | 24000 Hz |

RTF below 1.0 means synthesis outruns playback, so audio plays without gaps.
Kitten is retained as an option but is ~3x slower than realtime and will
stutter on long answers.

Kitten's model holds eight speakers; each is exposed under its published
name. Male: `jasper` `bruno` `hugo` `leo`. Female: `bella` `luna` `rosie`
`kiki`.

Switch at runtime from inside either agent — no restart:

```
/voice             # list voices, show the current one
/voice ryan        # switch
/voice luna        # switch
```

## The --serve protocol

One JSON request per stdin line, one JSON response per stdout line:

```
{"cmd":"speak","text":"..."}   queue an utterance      -> {"ok":true}
{"cmd":"voice"}                list voices             -> {"ok":true,"text":"..."}
{"cmd":"voice","name":"ryan"}  switch voice            -> {"ok":true,"text":"..."}
```

The child exits on stdin EOF, so it can never outlive the agent that
spawned it.

## Build & install

```bash
make fetch-tts-models      # bin/tts-models/ (~199 MB, all three voices)
make build-rex-tts         # bin/rex-tts/ (aarch64 binary + bundled sherpa libs)
make install-rex-tts-brick # engine + both extensions + voices into the chroot
```

The build is cgo (aarch64 cross-compile in the same Docker image rex uses for
`rex-brick`). It bundles `libsherpa-onnx-c-api.so` and `libonnxruntime.so`
beside the binary and links with `RUNPATH=$ORIGIN`, so the engine resolves
its own libraries wherever it lives.

Release tarballs bundle all of this (see `scripts/package.sh`); the make
target is for dev iteration against a device.

On-device layout:

```
~/.local/lib/rex-tts/                     binary + sherpa libs (canonical home)
~/.local/state/zot/extensions/zot-tts/    extension.json + symlink to the binary
~/.pi/agent/extensions/pi-tts.ts          pi extension (spawns the binary)
~/models/<voice-dir>/                     voice models, shared by everything
```

## Environment

- `REX_TTS_VOICE` — starting voice (default `lessac`)
- `REX_TTS_MODELS` — models root (default `~/models`), one subdir per voice
- `REX_TTS_SPEED` — speed multiplier (default `1.25`)
- `REX_TTS_THREADS` — inference threads (default 3)
- `REX_TTS_PLAYER` — playback command; `{rate}` is substituted with the
  loaded model's sample rate (default `aplay -q -f S16_LE -c 1 -t raw -r {rate}`)
- `REX_TTS_BIN` — (pi-tts only) path to the rex-tts binary, when it isn't at
  `~/.local/lib/rex-tts/rex-tts`

Logs: zot mode logs to `$ZOT_HOME/logs/ext-zot-tts.log`; serve mode logs to
stderr, which pi surfaces in its own output.
