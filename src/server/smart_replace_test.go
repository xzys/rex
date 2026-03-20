package server

import "testing"

func TestComputeDelta(t *testing.T) {
	tests := []struct {
		name       string
		sent       string
		newText    string
		wantBS     int
		wantToType string
	}{
		{
			name:       "empty to text",
			sent:       "",
			newText:    "hello",
			wantBS:     0,
			wantToType: "hello",
		},
		{
			name:       "append only",
			sent:       "hello",
			newText:    "hello wor",
			wantBS:     0,
			wantToType: " wor",
		},
		{
			name:       "append more",
			sent:       "hello wor",
			newText:    "hello world",
			wantBS:     0,
			wantToType: "ld",
		},
		{
			name:       "correction - replace suffix",
			sent:       "hello world",
			newText:    "hello word",
			wantBS:     2,
			wantToType: "d",
		},
		{
			name:       "full replacement",
			sent:       "abc",
			newText:    "xyz",
			wantBS:     3,
			wantToType: "xyz",
		},
		{
			name:       "text to empty",
			sent:       "hello",
			newText:    "",
			wantBS:     5,
			wantToType: "",
		},
		{
			name:       "both empty",
			sent:       "",
			newText:    "",
			wantBS:     0,
			wantToType: "",
		},
		{
			name:       "identical",
			sent:       "hello",
			newText:    "hello",
			wantBS:     0,
			wantToType: "",
		},
		{
			name:       "unicode append",
			sent:       "café",
			newText:    "café latte",
			wantBS:     0,
			wantToType: " latte",
		},
		{
			name:       "unicode correction",
			sent:       "naïve",
			newText:    "naïf",
			wantBS:     2,
			wantToType: "f",
		},
		{
			name:       "single char start",
			sent:       "",
			newText:    "h",
			wantBS:     0,
			wantToType: "h",
		},
		{
			name:       "complete rewrite short",
			sent:       "a",
			newText:    "b",
			wantBS:     1,
			wantToType: "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBS, gotToType := computeDelta(tt.sent, tt.newText)
			if gotBS != tt.wantBS || gotToType != tt.wantToType {
				t.Errorf("computeDelta(%q, %q) = (%d, %q), want (%d, %q)",
					tt.sent, tt.newText, gotBS, gotToType, tt.wantBS, tt.wantToType)
			}
		})
	}
}

// TestSmartReplaceSequence simulates a full transcription session
func TestSmartReplaceSequence(t *testing.T) {
	type step struct {
		newText    string
		wantBS     int
		wantToType string
	}

	steps := []step{
		{"hello", 0, "hello"},
		{"hello wor", 0, " wor"},
		{"hello world", 0, "ld"},
		{"hello word", 2, "d"},          // correction
		{"hello world today", 1, "ld today"}, // correction + append
	}

	sent := ""
	for i, s := range steps {
		bs, toType := computeDelta(sent, s.newText)
		if bs != s.wantBS || toType != s.wantToType {
			t.Errorf("step %d: computeDelta(%q, %q) = (%d, %q), want (%d, %q)",
				i, sent, s.newText, bs, toType, s.wantBS, s.wantToType)
		}
		// Apply the delta to simulate what would happen
		sent = sent[:len(sent)-bs] + toType
		if sent != s.newText {
			t.Errorf("step %d: after applying delta, sent=%q, want %q", i, sent, s.newText)
		}
	}
}
