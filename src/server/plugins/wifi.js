// wifi per-app key mapping.
//
// Gamepad bindings for the `rex wifi` TUI:
//   A  = connect (falls through to global → Enter)
//   B  = back    (falls through to global → Escape)
//   X  = disconnect
//   Y  = refresh / scan
//   R3 = forget
//
// X/Y/R3 send F-keys (not printable chars) so they don't get typed into
// the passphrase prompt buffer when one is open — F-keys aren't printable
// runes, so the password input ignores them.

function handleKey(key) {
  switch (key) {
    case KEY_X:  return "F4";
    case KEY_Y:  return "F5";
    case KEY_R3: return "F6";
  }
  return null;
}
