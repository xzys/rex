// lazygit per-app key mapping.
//
// The Brick's two front "FN" buttons (left/right of the d-pad area) drive
// lazygit's collapse/expand and context-line shortcuts:
//   left FN  (KEY_L3) -> '-'
//   right FN (KEY_R3) -> '+'
// Everything else falls through to the rex global fallback.

function handleKey(key) {
  switch (key) {
    case KEY_L3:
      return "-";
    case KEY_R3:
      return "+";
  }
  return null;
}
