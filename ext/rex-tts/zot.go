// The zot frontend observes assistant_message / turn_end events (no
// interception, so it never sits in the agent's critical path) and speaks the
// final answer of each turn.
package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/patriceckhart/zot/packages/agent/ext"
)

func runZot() int {
	e := ext.New("zot-tts", version)

	s, err := newSpeaker(e.Logf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rex-tts: %v\n", err)
		return 1
	}
	go s.loop()

	e.Command("voice", "list TTS voices, or switch: /voice ryan", func(args string) ext.Response {
		name := strings.TrimSpace(args)
		if name == "" {
			return ext.Display(s.describe())
		}
		if err := s.switchTo(name); err != nil {
			return ext.Display(fmt.Sprintf("rex-tts: %v", err))
		}
		return ext.Display("rex-tts: switched to " + name)
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
		s.enqueue(text)
	})

	runErr := e.Run()
	s.close()
	if runErr != nil {
		return 1
	}
	return 0
}
