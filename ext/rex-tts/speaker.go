// The speaker is the shared TTS core: the voice table, model loading and
// switching, and sentence-streamed synthesis into an external player.
//
// Synthesis runs in-process via sherpa-onnx, which keeps the model resident
// and hands back audio through a callback as each sentence is generated. On
// the Brick the piper voices run at ~0.8x realtime, so synthesis stays ahead
// of playback and the audio plays without gaps.
//
// All voices go through one code path: sherpa-onnx dispatches on whichever
// sub-config is populated, so switching voices at runtime is a config swap
// plus a model reload (under a second), not a different backend.
//
// Model loading never happens on a frontend's critical path. Constructing a
// speaker only reads configuration; the starting voice loads on a background
// goroutine (start), so zot's extension handshake and pi's --serve loop both
// answer immediately. Utterances that arrive before the model is up wait in
// the queue.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// A voice is a model directory plus the knowledge of which sherpa-onnx model
// family it belongs to. Everything downstream of NewOfflineTts is identical.
type voice struct {
	name string
	dir  string // directory under the models root
	desc string
	// kitten models need a voices.bin; the piper vits models are
	// single-speaker and always use sid 0.
	kitten bool
	sid    int
}

var voices = []voice{
	{name: "lessac", dir: "vits-piper-en_US-lessac-medium", desc: "piper en_US lessac (medium)"},
	{name: "ryan", dir: "vits-piper-en_US-ryan-medium", desc: "piper en_US ryan (medium)"},
}

// kitten ships eight speakers in one model; expose each under its published
// name. Speaker ids follow the model's expr-voice-N-{m,f} order.
func init() {
	for sid, k := range []struct{ name, internal string }{
		{"jasper", "expr-voice-2-m"},
		{"bella", "expr-voice-2-f"},
		{"bruno", "expr-voice-3-m"},
		{"luna", "expr-voice-3-f"},
		{"hugo", "expr-voice-4-m"},
		{"rosie", "expr-voice-4-f"},
		{"leo", "expr-voice-5-m"},
		{"kiki", "expr-voice-5-f"},
	} {
		voices = append(voices, voice{
			name:   k.name,
			dir:    "kitten-nano-en-v0_1-fp16",
			desc:   "kitten nano (" + k.internal + ")",
			kitten: true,
			sid:    sid,
		})
	}
}

func findVoice(name string) (voice, bool) {
	for _, v := range voices {
		if v.name == name {
			return v, true
		}
	}
	return voice{}, false
}

type speaker struct {
	queue  chan string
	logf   func(format string, args ...any)
	loaded chan struct{} // closed once the starting voice has been loaded (or failed)

	root    string
	speed   float32
	sid     int
	threads int
	player  string

	loadMu sync.Mutex // serialises model loads, so a /voice during startup can't race the initial one

	mu      sync.Mutex // guards everything below; held across a whole utterance
	tts     *sherpa.OfflineTts
	current string
	rate    int
}

// newSpeaker reads configuration only — no model is touched, so this returns
// in microseconds. Call start to load the starting voice and begin playback.
func newSpeaker(logf func(format string, args ...any)) *speaker {
	home, _ := os.UserHomeDir()
	s := &speaker{
		queue:   make(chan string, 8),
		logf:    logf,
		loaded:  make(chan struct{}),
		root:    envOr("REX_TTS_MODELS", filepath.Join(home, "models")),
		speed:   1.25,
		threads: 3,
		player:  envOr("REX_TTS_PLAYER", "aplay -q -f S16_LE -c 1 -t raw -r {rate}"),
	}
	if v := os.Getenv("REX_TTS_SPEED"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			s.speed = float32(f)
		}
	}
	if v := os.Getenv("REX_TTS_THREADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.threads = n
		}
	}
	return s
}

// start loads the starting voice in the background and runs the playback
// loop. It returns immediately: on the Brick a model load takes seconds, and
// zot gives extensions only a 3s grace to answer its handshake before it
// gives up and carries on without them.
func (s *speaker) start() {
	go func() {
		defer close(s.loaded)
		s.loadMu.Lock()
		defer s.loadMu.Unlock()
		if s.voice() != "" {
			return // a /voice command beat us to it; don't undo the user's choice
		}
		if err := s.load(envOr("REX_TTS_VOICE", "lessac")); err != nil {
			s.logf("%v", err)
		}
	}()
	go s.loop()
}

// voice reports the loaded voice, empty while the model is still loading.
func (s *speaker) voice() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// switchTo loads a voice, replacing any currently-loaded one. Model load is
// well under a second once the page cache is warm, so this is cheap enough to
// do from a slash command.
func (s *speaker) switchTo(name string) error {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	return s.load(name)
}

// load does the work of switchTo with loadMu already held.
func (s *speaker) load(name string) error {
	v, ok := findVoice(name)
	if !ok {
		return fmt.Errorf("unknown voice %q (have: %s)", name, strings.Join(voiceNames(), ", "))
	}
	dir := filepath.Join(s.root, v.dir)
	cfg, err := buildConfig(v, dir, s.threads)
	if err != nil {
		return err
	}

	tts := sherpa.NewOfflineTts(cfg)
	if tts == nil {
		return fmt.Errorf("failed to load voice %q from %s", name, dir)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tts != nil {
		sherpa.DeleteOfflineTts(s.tts)
	}
	s.tts = tts
	s.current = name
	s.sid = v.sid
	// Sample rates differ per model (piper 22050, kitten 24000), so the
	// player has to be told the rate of whatever is loaded right now.
	s.rate = tts.SampleRate()
	if s.sid >= tts.NumSpeakers() {
		s.sid = 0
	}
	return nil
}

func buildConfig(v voice, dir string, threads int) (*sherpa.OfflineTtsConfig, error) {
	model, err := findModel(dir, v.kitten)
	if err != nil {
		return nil, err
	}
	tokens := filepath.Join(dir, "tokens.txt")
	data := filepath.Join(dir, "espeak-ng-data")
	for _, p := range []string{tokens, data} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("voice %q: missing %s", v.name, p)
		}
	}

	cfg := &sherpa.OfflineTtsConfig{}
	cfg.Model.NumThreads = threads
	cfg.Model.Provider = "cpu"
	if v.kitten {
		cfg.Model.Kitten = sherpa.OfflineTtsKittenModelConfig{
			Model:       model,
			Voices:      filepath.Join(dir, "voices.bin"),
			Tokens:      tokens,
			DataDir:     data,
			LengthScale: 1.0,
		}
		if _, err := os.Stat(cfg.Model.Kitten.Voices); err != nil {
			return nil, fmt.Errorf("voice %q: missing %s", v.name, cfg.Model.Kitten.Voices)
		}
	} else {
		cfg.Model.Vits = sherpa.OfflineTtsVitsModelConfig{
			Model:       model,
			Tokens:      tokens,
			DataDir:     data,
			NoiseScale:  0.667,
			NoiseScaleW: 0.8,
			LengthScale: 1.0,
		}
	}
	return cfg, nil
}

func findModel(dir string, kitten bool) (string, error) {
	if kitten {
		p := filepath.Join(dir, "model.fp16.onnx")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.onnx"))
	if len(matches) == 0 {
		return "", fmt.Errorf("no .onnx model in %s", dir)
	}
	sort.Strings(matches)
	return matches[0], nil
}

func (s *speaker) describe() string {
	s.mu.Lock()
	cur, rate := s.current, s.rate
	s.mu.Unlock()

	var b strings.Builder
	if cur == "" {
		fmt.Fprintf(&b, "rex-tts voices (still loading, speed %.2f)\n", s.speed)
	} else {
		fmt.Fprintf(&b, "rex-tts voices (current: %s @ %d Hz, speed %.2f)\n", cur, rate, s.speed)
	}
	for _, v := range voices {
		mark := "  "
		if v.name == cur {
			mark = "* "
		}
		installed := ""
		if _, err := os.Stat(filepath.Join(s.root, v.dir)); err != nil {
			installed = "  [not installed]"
		}
		fmt.Fprintf(&b, "%s%-11s %s%s\n", mark, v.name, v.desc, installed)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *speaker) loop() {
	<-s.loaded // anything said during startup waits in the queue
	for text := range s.queue {
		if err := s.speak(text); err != nil {
			s.logf("speak: %v", err)
		}
	}
}

// enqueue hands an utterance to the playback loop without blocking; when the
// queue is full the utterance is dropped rather than stalling the caller.
func (s *speaker) enqueue(text string) {
	select {
	case s.queue <- text:
	default:
		s.logf("speech queue full, dropping response")
	}
}

// speak synthesizes and plays one utterance. sherpa-onnx invokes the callback
// once per sentence as that sentence's audio becomes available, so playback
// starts after the first sentence rather than the whole answer.
func (s *speaker) speak(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tts == nil {
		return fmt.Errorf("no voice loaded (the starting voice failed to load; try /voice)")
	}

	args := strings.Fields(strings.ReplaceAll(s.player, "{rate}", strconv.Itoa(s.rate)))
	play := exec.Command(args[0], args[1:]...)
	play.Stderr = os.Stderr
	pcm, err := play.StdinPipe()
	if err != nil {
		return err
	}
	if err := play.Start(); err != nil {
		return err
	}

	var writeErr error
	s.tts.GenerateWithCallback(text, s.sid, s.speed, func(samples []float32) bool {
		if _, err := pcm.Write(pcmBytes(samples)); err != nil {
			writeErr = err
			return false // player died — stop generating
		}
		return true
	})

	pcm.Close() // EOF tells the player the utterance is over
	if waitErr := play.Wait(); writeErr == nil {
		writeErr = waitErr
	}
	return writeErr
}

// pcmBytes converts sherpa's normalized float samples to S16_LE.
func pcmBytes(samples []float32) []byte {
	buf := make([]byte, 0, len(samples)*2)
	for _, f := range samples {
		v := f * 32767
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		i := int16(v)
		buf = append(buf, byte(i), byte(i>>8))
	}
	return buf
}

func (s *speaker) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tts != nil {
		sherpa.DeleteOfflineTts(s.tts)
		s.tts = nil
	}
}

func voiceNames() []string {
	out := make([]string, 0, len(voices))
	for _, v := range voices {
		out = append(out, v.name)
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
