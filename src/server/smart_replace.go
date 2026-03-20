package server

// computeDelta computes the minimal edit to transform sent into newText.
// Returns the number of backspaces to delete from the end of sent,
// and the string to type after those deletions.
func computeDelta(sent, newText string) (backspaces int, toType string) {
	// Find longest common prefix
	common := 0
	for common < len(sent) && common < len(newText) && sent[common] == newText[common] {
		common++
	}
	backspaces = len(sent) - common
	toType = newText[common:]
	return
}
