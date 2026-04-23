// menu per-app key mapping.
//
// The global operator remaps the d-pad (up/down = Up/Down arrows, left/right =
// tmux scroll up/down) for terminal navigation. The menu is a grid and needs
// the literal axes, so translate each d-pad direction straight to its matching
// arrow key.

function handleKey(key) {
  switch (key) {
    case KEY_DPAD_UP:    return "Up";
    case KEY_DPAD_DOWN:  return "Down";
    case KEY_DPAD_LEFT:  return "Left";
    case KEY_DPAD_RIGHT: return "Right";
  }
  return null;
}
