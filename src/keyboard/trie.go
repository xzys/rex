package keyboard

import (
	"bufio"
	"io"
	"strings"
)

type TrieNode struct {
	children [26]*TrieNode
	word     string // non-empty if this node completes a word
	rank     int    // lower = more frequent (0 = most frequent)
}

type Trie struct {
	root *TrieNode
}

func NewTrie() *Trie {
	return &Trie{root: &TrieNode{}}
}

func (t *Trie) Insert(word string, rank int) {
	node := t.root
	for _, ch := range strings.ToLower(word) {
		idx := ch - 'a'
		if idx < 0 || idx >= 26 {
			return // skip non-alpha words
		}
		if node.children[idx] == nil {
			node.children[idx] = &TrieNode{}
		}
		node = node.children[idx]
	}
	node.word = strings.ToLower(word)
	node.rank = rank
}

// Search returns up to limit words matching the prefix, ordered by frequency rank (lowest rank first).
func (t *Trie) Search(prefix string, limit int) []string {
	if prefix == "" || limit <= 0 {
		return nil
	}

	// Navigate to prefix node
	node := t.root
	for _, ch := range strings.ToLower(prefix) {
		idx := ch - 'a'
		if idx < 0 || idx >= 26 {
			return nil
		}
		if node.children[idx] == nil {
			return nil
		}
		node = node.children[idx]
	}

	// Collect all words under this node
	var candidates []struct {
		word string
		rank int
	}
	var collect func(n *TrieNode)
	collect = func(n *TrieNode) {
		if n.word != "" {
			candidates = append(candidates, struct {
				word string
				rank int
			}{n.word, n.rank})
		}
		for _, child := range n.children {
			if child != nil {
				collect(child)
			}
		}
	}
	collect(node)

	// Selection sort for top-limit by rank (candidates are typically small for short prefixes)
	results := make([]string, 0, limit)
	for i := 0; i < limit && i < len(candidates); i++ {
		minIdx := i
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].rank < candidates[minIdx].rank {
				minIdx = j
			}
		}
		candidates[i], candidates[minIdx] = candidates[minIdx], candidates[i]
		results = append(results, candidates[i].word)
	}
	return results
}

// LoadTrie reads a frequency-sorted word list (one word per line, most frequent first)
// and returns a populated Trie. maxWords limits how many words to load (0 = all).
func LoadTrie(r io.Reader, maxWords int) *Trie {
	trie := NewTrie()
	scanner := bufio.NewScanner(r)
	rank := 0
	for scanner.Scan() {
		if maxWords > 0 && rank >= maxWords {
			break
		}
		word := strings.TrimSpace(scanner.Text())
		if word == "" {
			continue
		}
		trie.Insert(word, rank)
		rank++
	}
	return trie
}
