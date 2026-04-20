// ncmpcpp per-app key mapping.
//
// Cycles through 8 ncmpcpp screens via d-pad left/right and proxies transport
// controls to ncmpcpp's own shortcuts. Return null to fall through to the
// rex global fallback (A=Enter, B=Escape, d-pad nav, etc.).

let screen = 0;

function handleKey(key) {
  switch (key) {
    case KEY_DPAD_RIGHT:
      screen = (screen + 1) % 8;
      return String(screen + 1);
    case KEY_DPAD_LEFT:
      screen = (screen - 1 + 8) % 8;
      return String(screen + 1);
    case KEY_B:
      return "p";
    case KEY_X:
      return ">";
    case KEY_Y:
      return "<";
  }
  return null;
}
