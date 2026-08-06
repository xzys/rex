// The zot frontend observes assistant_message events (no interception, so it
// never sits in the agent's critical path) and speaks each one as it arrives —
// interim narration between tool calls as well as the final answer. Thinking
// and tool blocks never reach the event text: zot's host assembles it from
// TextBlock content only.
package main

import (
	"fmt"
	"os"
	"strings"

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

	e.On("assistant_message", func(ev ext.Event) {
		if t := strings.TrimSpace(ev.Text); t != "" {
			s.enqueue(t)
		}
	})

	runErr := e.Run()
	s.close()
	if runErr != nil {
		return 1
	}
	return 0
}
