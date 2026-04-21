// menu per-app key mapping.
//
// The global operator remaps the d-pad (up/down = tmux scroll, left/right =
// Up/Down arrows) because most apps use the main pane for scrollback reading.
// The menu is a grid and needs the literal axes, so translate each d-pad
// direction straight to its matching arrow key.

function handleKey(key) {
  switch (key) {
    case KEY_DPAD_UP:    return "Up";
    case KEY_DPAD_DOWN:  return "Down";
    case KEY_DPAD_LEFT:  return "Left";
    case KEY_DPAD_RIGHT: return "Right";
  }
  return null;
}
