package keyboard

import (
	"strings"

	"github.com/xrfr/t-rex/v2/protocol"
)

// This file owns keyboard *behavior* that is identical across every input
// model: the special-key labels, the numbers/symbol layer, the shift layer,
// and the resolution from an on-screen key label to the bytes sent to the
// server. Input models (simple, front-chorded) decide only *how* a base key is
// selected — they delegate "what does this key produce" to layerState here so
// the two stay in lockstep.

// specialKeyMap maps on-screen key labels to the special-key runes the server
// understands. "123" and "SFT" are intentionally absent: they are layer
// toggles handled locally by layerState, never sent to the server (which
// rejects the SpecialKeyNumbers control code outright).
var specialKeyMap = map[string]rune{
	"SPC": protocol.SpecialKeySpace,
	"DEL": protocol.SpecialKeyDelete,
	"MIC": protocol.SpecialKeyMic,
	"ENT": protocol.SpecialKeyEnter,
}

// numbersLayer is the symbol/number layer reached via the "123" toggle. Keyed
// on the base (lowercase) letter so it applies uniformly to any layout built
// from the standard QWERTY letter set. Top row gives digits; the home and
// bottom rows give common symbols for terminal/coding use.
var numbersLayer = map[string]string{
	"q": "1", "w": "2", "e": "3", "r": "4", "t": "5",
	"y": "6", "u": "7", "i": "8", "o": "9", "p": "0",
	"a": "@", "s": "#", "d": "$", "f": "_", "g": "&",
	"h": "-", "j": "+", "k": "(", "l": ")",
	"z": "*", "x": "\"", "c": "'", "v": ":", "b": ";",
	"n": "!", "m": "?",
}

// layerState holds the active shift/numbers layers. Both toggles are sticky:
// they stay engaged until pressed again.
type layerState struct {
	numbers bool
	shift   bool
}

// toggle applies the "123"/"SFT" layer toggles. Returns true when `label` is a
// toggle, meaning it was consumed and nothing should be sent to the server.
func (l *layerState) toggle(label string) bool {
	switch label {
	case "123":
		l.numbers = !l.numbers
		return true
	case "SFT":
		l.shift = !l.shift
		return true
	}
	return false
}

// resolve maps a base key `label` to the string sent to the server under the
// current layers. Returns nil when the label is a consumed layer toggle or is
// unknown. Numbers takes precedence over shift.
func (l *layerState) resolve(label string) *string {
	if l.toggle(label) {
		return nil
	}
	if mapped, ok := specialKeyMap[label]; ok {
		s := string(mapped)
		return &s
	}
	out := label
	if l.numbers {
		if n, ok := numbersLayer[label]; ok {
			out = n
		}
	} else if l.shift {
		out = strings.ToUpper(label)
	}
	return &out
}

// glyph returns what to display for base key `label` under the current layers.
func (l layerState) glyph(label string) string {
	if _, ok := specialKeyMap[label]; ok {
		return label
	}
	switch label {
	case "123", "SFT":
		return label
	}
	if l.numbers {
		if n, ok := numbersLayer[label]; ok {
			return n
		}
		return label
	}
	if l.shift {
		return strings.ToUpper(label)
	}
	return label
}

// active reports whether `label` is a layer toggle that is currently engaged,
// so the renderer can highlight it.
func (l layerState) active(label string) bool {
	return (label == "123" && l.numbers) || (label == "SFT" && l.shift)
}
