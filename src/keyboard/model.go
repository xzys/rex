package keyboard

type KeyboardModel interface {
	// modify internal state based on key
	// returns the key to press or nil
	HandleKey(key int, isKeyDown bool) *string
	// takes key and position
	// returns foreground, background, key (same or override)
	RenderKey(i, j int, key string) (string, string, string)
}

// VoiceRenderer is implemented by models that render a full-screen view
// instead of the keyboard grid (e.g. voice input mode).
type VoiceRenderer interface {
	RenderView(width int) string
}
