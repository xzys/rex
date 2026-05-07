# Architecture

## System Architecture

```
┌──────────┐
│   R36S   │
└────┬─────┘
     │ SDL events
┌────▼─────────────────────────────────────────────────────┐
│                          Daemon                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  SDL Input Handler                                 │  │
│  │  - Polls gamepad                                   │  │
│  │  - When keyboard active: sends KeyInputMessage     │  │
│  └───────────┬────────────────────────────────────────┘  │
│              │                                           │
│  ┌───────────▼────────────────────────────────────────┐  │
│  │  Connection Handler                                │  │
│  │  - Sends input_key to keyboard                     │  │
│  │  - Receives key_press and executes operation       │  │
│  └───────────┬────────────────────────────────────────┘  │
└──────────────┼────────────────────────▲──────────────────┘
               │                        |
               │  input_key             │  key_press
               |                        │
┌──────────────▼────────────────────────┼──────────────────┐
│                        Keyboard                          │
│  - Manages state: modifiers, modes (shift, numbers)      │
│  - Simple / Chorded UX modes                             |
└──────────────────────────────────────────────────────────┘
```

## Components

### Server (`server/`)

- Manages input device polling (SDL gamepad or keyboard)
- Accepts keyboard client connections via Unix sockets:
  - `/tmp/rex-input.sock` - server → keyboard (KeyInput messages)
  - `/tmp/rex-output.sock` - keyboard → server (KeyPress messages)
- Forwards device input to keyboard client as `KeyInputMessage`
- Receives `PressKeyMessage` from keyboard client and executes key presses via tmux

### Keyboard Client (`keyboard/`)

- Visual keyboard interface using Bubble Tea
- Connects to server and receives device input
- Translates gamepad inputs to characters using chorded or simple model
- Sends translated characters back to server as `PressKeyMessage`

### Start Menu (`menu/`)

- Grid-based app launcher with configurable apps
- Uses arrow keys/D-pad to navigate, Enter/A to select

### Protocol (`protocol/`)

- Defines key constants (D-Pad, face buttons, modifiers)
- Special keys use non-printable control codes (`\x01`-`\x04`) to avoid collisions
- JSON messages over Unix socket, line-delimited

### Operators (`server/operator*.go`)

- Handle app-specific key bindings when keyboard is not active. Operator is
  selected by the current tmux window name.
- `GlobalOperator`: Manages tmux windows and routes keys to active operator. Fallback bindings when no active operator claims the key: A=Enter, B=Escape, D-Pad Up/Down=Up/Down arrows, D-Pad Left/Right=scroll page up/down, Select=open keyboard, Start=open menu
- `ScriptOperator` (`operator_script.go`, `plugins.go`): loads JS plugins from
  `~/.config/rex/plugins/<window>.js` via `buke/quickjs-go` (CGo QuickJS).
  Plugins export a global `handleKey(key)` and return a string for
  `tmux send-keys -t 0` or `null` to fall through. Defaults are embedded via
  `go:embed plugins/*.js` and used when a user plugin is missing. Current
  defaults: `ncmpcpp.js`. See `notes/features/js-plugins.md`.
- `SSHTmuxOperator` (`tmux` window): stays in Go — runs remote `tmux`
  commands over the ControlMaster socket, not pure key→string.
- `ClaudeOperator` (`claude` window): stays in Go — owns async timers
  (voice PTT repeat, backspace autorepeat) and an audio relay. Y=voice PTT,
  X=Escape, B=backspace with repeat, D-Pad Up/Down=Up/Down arrows,
  D-Pad Left/Right=scroll page up/down.

### Keyboard Models

- **Chorded**: Modifier + trigger combinations (L1/R1/L2 + D-Pad/buttons), supports shift and numbers modes
- **Simple**: Move to key and press A to select

See `notes/keyboard-modes.md` for detailed mappings.

## Data Flow

1. Input device → Server (`HandleKey`) → `KeyInputMessage` → Keyboard client
2. Keyboard client → Model (`HandleKey`) → Character → `PressKeyMessage` → Server
3. Server → `HandleKeyPress()` → Execute key press via tmux

## Stdin Input Mode

Set `REX_INPUT_DEVICE=stdin` to read input events from stdin. Each line is:

```
down|up <key>
```

Key tokens are case-insensitive and accept an optional `Key` prefix:
`KeyA`, `KeyB`, `KeyX`, `KeyY`, `KeyDPadUp`, `KeyDPadDown`, `KeyDPadLeft`, `KeyDPadRight`,
`KeyL1`, `KeyR1`, `KeyL2`, `KeyR2`, `KeySelect`, `KeyStart`, `KeyFn`.
`up`, `down`, `left`, `right` are accepted as D-pad aliases.

Manual harness:

```
./scripts/test_stdin_input.sh
```

## Logging

Logs are written to `/tmp/rex.log` with debug-level output.

## Design Decisions

- **Select**: Use this to open/close the keyboard app
- **Synchronous connections**: Only one keyboard allowed at a time
- **Tmux integration**: All key presses are executed via tmux send-keys to pane 0
