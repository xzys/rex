package keyboard

import (
	"strings"
	"testing"
)

func TestTrieSearch(t *testing.T) {
	trie := NewTrie()
	// Insert words in frequency order (rank = insertion order)
	words := []string{"the", "they", "there", "then", "these", "them", "this", "think", "time"}
	for i, w := range words {
		trie.Insert(w, i)
	}

	tests := []struct {
		prefix   string
		limit    int
		expected []string
	}{
		{"th", 4, []string{"the", "they", "there", "then"}},
		{"the", 4, []string{"the", "they", "there", "then"}},
		{"them", 4, []string{"them"}},
		{"ti", 4, []string{"time"}},
		{"z", 4, nil},
		{"", 4, nil},
		{"th", 2, []string{"the", "they"}},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			got := trie.Search(tc.prefix, tc.limit)
			if len(got) != len(tc.expected) {
				t.Fatalf("Search(%q, %d) = %v, want %v", tc.prefix, tc.limit, got, tc.expected)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("Search(%q, %d)[%d] = %q, want %q", tc.prefix, tc.limit, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestLoadTrie(t *testing.T) {
	input := "the\nthey\nthere\nthen\nthis\n"
	trie := LoadTrie(strings.NewReader(input), 0)

	got := trie.Search("th", 4)
	expected := []string{"the", "they", "there", "then"}
	if len(got) != len(expected) {
		t.Fatalf("got %v, want %v", got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}
