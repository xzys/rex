package keyboard

type KeyboardModel interface {
	// modify internal state based on key
	// returns the key to press (empty string if none) and error
	HandleKey(key int) (*string, error)
	// takes key and position
	// returns foreground, background, key (same or override)
	RenderKey(i, j int, key string) (string, string, string)
}
