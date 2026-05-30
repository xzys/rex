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

// numbersLayer is the first symbol page, reached via the "123" toggle. Keyed
// on the base (lowercase) letter so it applies uniformly to any layout built
// from the standard QWERTY letter set. Top row gives digits; the home and
// bottom rows give the most common symbols for terminal/coding use. The
// remaining ASCII punctuation lives on symbolsLayer (page 2).
var numbersLayer = map[string]string{
	"q": "1", "w": "2", "e": "3", "r": "4", "t": "5",
	"y": "6", "u": "7", "i": "8", "o": "9", "p": "0",
	"a": "@", "s": "#", "d": "$", "f": "_", "g": "&",
	"h": "-", "j": "+", "k": "(", "l": ")",
	"z": "*", "x": "\"", "c": "'", "v": ":", "b": ";",
	"n": "!", "m": "?",
}

// symbolsLayer is the second symbol page, reached by pressing "SFT" while on
// the numbers page (iOS "#+=" style). It holds the punctuation that didn't fit
// on page 1, so between the two pages every common ASCII symbol is reachable.
// Keys absent here are inert on this page.
var symbolsLayer = map[string]string{
	"q": "`", "w": "~", "e": "%", "r": "^", "t": "=",
	"y": "[", "u": "]", "i": "{", "o": "}", "p": "\\",
	"a": "|", "s": "/", "d": "<", "f": ">", "g": ",",
	"h": ".",
}

// kbLayer is the active character page.
type kbLayer int

const (
	layerLetters kbLayer = iota // a-z, optionally uppercased by shift
	layerNum1                   // digits + common symbols (numbersLayer)
	layerNum2                   // extended symbols (symbolsLayer)
)

// layerState holds the active page and shift state. Toggles are sticky: a page
// stays engaged until switched away. Page transitions (iOS-style):
//
//	letters --123--> num1 <--SFT--> num2
//	   ^-------123----------|----------/
//
// i.e. "123" flips letters<->num1 (and num2->letters), and "SFT" caps-toggles
// in letters but pages between num1 and num2 while in numbers.
type layerState struct {
	layer kbLayer
	shift bool
}

// toggle applies the "123"/"SFT" layer toggles. Returns true when `label` is a
// toggle, meaning it was consumed and nothing should be sent to the server.
func (l *layerState) toggle(label string) bool {
	switch label {
	case "123":
		if l.layer == layerLetters {
			l.layer = layerNum1
		} else {
			l.layer = layerLetters
		}
		return true
	case "SFT":
		switch l.layer {
		case layerLetters:
			l.shift = !l.shift
		case layerNum1:
			l.layer = layerNum2
		case layerNum2:
			l.layer = layerNum1
		}
		return true
	}
	return false
}

// resolve maps a base key `label` to the string sent to the server under the
// current page. Returns nil when the label is a consumed layer toggle, an
// inert page-2 key, or unknown.
func (l *layerState) resolve(label string) *string {
	if l.toggle(label) {
		return nil
	}
	if mapped, ok := specialKeyMap[label]; ok {
		s := string(mapped)
		return &s
	}
	switch l.layer {
	case layerNum1:
		out := label
		if n, ok := numbersLayer[label]; ok {
			out = n
		}
		return &out
	case layerNum2:
		if s, ok := symbolsLayer[label]; ok {
			return &s
		}
		return nil // inert key on the symbols page
	default:
		out := label
		if l.shift {
			out = strings.ToUpper(label)
		}
		return &out
	}
}

// glyph returns what to display for base key `label` under the current page.
// The "123"/"SFT" toggle keys relabel to hint where they lead.
func (l layerState) glyph(label string) string {
	switch label {
	case "123":
		if l.layer == layerLetters {
			return "123"
		}
		return "abc"
	case "SFT":
		switch l.layer {
		case layerNum1:
			return "#+="
		case layerNum2:
			return "123"
		default:
			return "SFT"
		}
	}
	if _, ok := specialKeyMap[label]; ok {
		return label
	}
	switch l.layer {
	case layerNum1:
		if n, ok := numbersLayer[label]; ok {
			return n
		}
		return label
	case layerNum2:
		if s, ok := symbolsLayer[label]; ok {
			return s
		}
		return " " // inert key renders blank
	default:
		if l.shift {
			return strings.ToUpper(label)
		}
		return label
	}
}

// active reports whether `label` is a layer toggle that is currently engaged,
// so the renderer can highlight it.
func (l layerState) active(label string) bool {
	switch label {
	case "123":
		return l.layer != layerLetters
	case "SFT":
		return l.shift || l.layer == layerNum2
	}
	return false
}
