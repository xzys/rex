// zot-tts speaks zot's final answers through on-device neural TTS.
//
// It observes assistant_message / turn_end events (no interception, so it
// never sits in the agent's critical path) and synthesizes the final answer
// of each turn.
//
// Synthesis runs in-process via sherpa-onnx, which keeps the model resident
// and hands back audio through a callback as each sentence is generated. On
// the Brick the piper voices run at ~0.8x realtime, so synthesis stays ahead
// of playback and the audio plays without gaps.
//
// All voices go through one code path: sherpa-onnx dispatches on whichever
// sub-config is populated, so switching voices at runtime is a config swap
// plus a model reload (under a second), not a different backend.
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
	"github.com/patriceckhart/zot/packages/agent/ext"
)

const version = "0.2.0"

// A voice is a model directory plus the knowledge of which sherpa-onnx model
// family it belongs to. Everything downstream of NewOfflineTts is identical.
type voice struct {
	name string
	dir  string // directory under the models root
	desc string
	// kitten models need a voices.bin and carry several speakers; the piper
	// vits models are single-speaker.
	kitten bool
}

var voices = []voice{
	{name: "lessac", dir: "vits-piper-en_US-lessac-medium", desc: "piper en_US lessac (medium)"},
	{name: "ryan", dir: "vits-piper-en_US-ryan-medium", desc: "piper en_US ryan (medium)"},
	{name: "kitten", dir: "kitten-nano-en-v0_1-fp16", desc: "kitten nano (fp16, 8 speakers)", kitten: true},
}

func findVoice(name string) (voice, bool) {
	for _, v := range voices {
		if v.name == name {
			return v, true
		}
	}
	return voice{}, false
}

func main() {
	e := ext.New("zot-tts", version)

	s, err := newSpeaker()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zot-tts: %v\n", err)
		os.Exit(1)
	}
	go s.loop(e)

	e.Command("voice", "list TTS voices, or switch: /voice ryan", func(args string) ext.Response {
		name := strings.TrimSpace(args)
		if name == "" {
			return ext.Display(s.describe())
		}
		if err := s.switchTo(name); err != nil {
			return ext.Display(fmt.Sprintf("zot-tts: %v", err))
		}
		return ext.Display("zot-tts: switched to " + name)
	})

	var (
		mu   sync.Mutex
		last string
	)
	e.On("assistant_message", func(ev ext.Event) {
		if t := strings.TrimSpace(ev.Text); t != "" {
			mu.Lock()
			last = t
			mu.Unlock()
		}
	})
	e.On("turn_end", func(ev ext.Event) {
		if ev.Stop != "end" {
			return
		}
		mu.Lock()
		text := last
		last = ""
		mu.Unlock()
		if text == "" {
			return
		}
		select {
		case s.queue <- text:
		default:
			e.Logf("speech queue full, dropping response")
		}
	})

	runErr := e.Run()
	s.close()
	if runErr != nil {
		os.Exit(1)
	}
}

type speaker struct {
	queue chan string

	root    string
	speed   float32
	sid     int
	threads int
	player  string

	mu      sync.Mutex // guards everything below; held across a whole utterance
	tts     *sherpa.OfflineTts
	current string
	rate    int
}

func newSpeaker() (*speaker, error) {
	home, _ := os.UserHomeDir()
	s := &speaker{
		queue:   make(chan string, 8),
		root:    envOr("ZOT_TTS_MODELS", filepath.Join(home, "models")),
		speed:   1.25,
		threads: 4,
		player:  envOr("ZOT_TTS_PLAYER", "aplay -q -f S16_LE -c 1 -t raw -r {rate}"),
	}
	if v := os.Getenv("ZOT_TTS_SPEED"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			s.speed = float32(f)
		}
	}
	if v := os.Getenv("ZOT_TTS_SID"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.sid = n
		}
	}
	if v := os.Getenv("ZOT_TTS_THREADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.threads = n
		}
	}
	if err := s.switchTo(envOr("ZOT_TTS_VOICE", "lessac")); err != nil {
		return nil, err
	}
	return s, nil
}

// switchTo loads a voice, replacing any currently-loaded one. Model load is
// well under a second, so this is cheap enough to do from a slash command.
func (s *speaker) switchTo(name string) error {
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
	fmt.Fprintf(&b, "zot-tts voices (current: %s @ %d Hz, speed %.2f)\n", cur, rate, s.speed)
	for _, v := range voices {
		mark := "  "
		if v.name == cur {
			mark = "* "
		}
		installed := ""
		if _, err := os.Stat(filepath.Join(s.root, v.dir)); err != nil {
			installed = "  [not installed]"
		}
		fmt.Fprintf(&b, "%s%-8s %s%s\n", mark, v.name, v.desc, installed)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *speaker) loop(e *ext.Extension) {
	for text := range s.queue {
		if err := s.speak(text); err != nil {
			e.Logf("speak: %v", err)
		}
	}
}

// speak synthesizes and plays one utterance. sherpa-onnx invokes the callback
// once per sentence as that sentence's audio becomes available, so playback
// starts after the first sentence rather than the whole answer.
func (s *speaker) speak(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tts == nil {
		return fmt.Errorf("no voice loaded")
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
