// rex-tts speaks a coding agent's final answers through on-device neural TTS.
//
// One binary, two frontends:
//
//   - default: a zot extension speaking zot's answers (zot.go)
//   - --serve: a JSON-lines stdio service for other agents; pi-tts.ts spawns
//     it as a long-lived child so the model stays resident (serve.go)
//
// The synthesis core is shared (speaker.go).
package main

import (
	"flag"
	"os"
)

const version = "0.4.0"

func main() {
	serve := flag.Bool("serve", false, "speak JSON-lines requests from stdin (for pi-tts)")
	flag.Parse()

	if *serve {
		os.Exit(runServe())
	}
	os.Exit(runZot())
}
