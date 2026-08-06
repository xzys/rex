// The serve frontend reads one JSON request per stdin line and answers with
// one JSON response per stdout line. It exists for agents that can't link the
// synthesis core in-process: a thin extension (pi-tts.ts) spawns
// `rex-tts --serve` as a long-lived child, so the model stays resident and
// the child dies with the agent.
//
// Requests:
//
//	{"cmd":"speak","text":"..."}   queue an utterance      -> {"ok":true}
//	{"cmd":"voice"}                list voices             -> {"ok":true,"text":"..."}
//	{"cmd":"voice","name":"ryan"}  switch voice            -> {"ok":true,"text":"..."}
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type request struct {
	Cmd  string `json:"cmd"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
}

type response struct {
	OK    bool   `json:"ok"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

func runServe() int {
	logger := log.New(os.Stderr, "rex-tts: ", log.LstdFlags)

	s, err := newSpeaker(logger.Printf)
	if err != nil {
		logger.Printf("%v", err)
		return 1
	}
	go s.loop()
	defer s.close()

	out := json.NewEncoder(os.Stdout)
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024) // answers can exceed the default 64K token limit

	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			out.Encode(response{Error: fmt.Sprintf("bad request: %v", err)})
			continue
		}
		out.Encode(handle(s, req))
	}
	if err := in.Err(); err != nil {
		logger.Printf("stdin: %v", err)
		return 1
	}
	return 0 // EOF: the spawning agent exited
}

func handle(s *speaker, req request) response {
	switch req.Cmd {
	case "speak":
		if req.Text == "" {
			return response{Error: "speak: empty text"}
		}
		s.enqueue(req.Text)
		return response{OK: true}
	case "voice":
		if req.Name == "" {
			return response{OK: true, Text: s.describe()}
		}
		if err := s.switchTo(req.Name); err != nil {
			return response{Error: err.Error()}
		}
		return response{OK: true, Text: "switched to " + req.Name}
	default:
		return response{Error: fmt.Sprintf("unknown cmd %q", req.Cmd)}
	}
}
