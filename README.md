# Architecture Overview

## Overview

REX is a client-server keyboard input system designed for the R36S handheld gaming device. It enables gamepad/joystick input to be translated into keyboard characters through a chorded keyboard interface. The system consists of two main components: a **server** that handles input device events and a **keyboard client** that provides a visual keyboard interface and translates gamepad inputs into characters.

## Usage

```
rex server                    # Start the daemon
rex keyboard [--mode MODE]    # Start keyboard (mode: chord or simple)
rex menu [--config PATH]      # Start menu (default: config.yaml)
```

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

### Start Menu (`start_menu/`)
- Grid-based app launcher with configurable apps
- Configured via YAML file (see `config.yaml`)
- Uses arrow keys/D-pad to navigate, Enter/A to select

### Protocol (`protocol/`)
- Defines key constants (D-Pad, face buttons, modifiers)
- Special keys use non-printable control codes (`\x01`-`\x04`) to avoid collisions
- JSON messages over Unix socket, line-delimited

### Operators (`server/operator*.go`)
- Handle app-specific key bindings when keyboard is not active
- `GlobalOperator`: Manages tmux windows and routes keys to active operator
- `NcmpcppOperator`: Music player controls (A=select, B=play/pause, X=skip, Y=back)

### Keyboard Models
- **Chorded**: Modifier + trigger combinations (L1/R1/L2 + D-Pad/buttons), supports shift and numbers modes
- **Simple**: Move to key and press A to select

## Global Key Bindings

When keyboard is **not** active:
| Key | Action |
|-----|--------|
| R2 | Open keyboard pane |
| R1 | Next tmux window |
| L1 | Previous tmux window |
| L2 | New tmux window |
| D-Pad | Arrow keys (sent to tmux) |

When keyboard **is** active:
| Key | Action |
|-----|--------|
| R2 | Close keyboard pane |
| All others | Sent to keyboard for chord processing |

## Development Keyboard Mappings

For local development without a gamepad (`make build-local`):

| Keyboard | Gamepad |
|----------|---------|
| Arrow keys | D-Pad |
| m | Y |
| , | X |
| . | B |
| / | A |
| z | L1 |
| x | R1 |
| c | L2 |
| v | R2 |

## Data Flow

1. Input device → Server (`HandleKey`) → `KeyInputMessage` → Keyboard client
2. Keyboard client → Model (`HandleKey`) → Character → `PressKeyMessage` → Server
3. Server → `HandleKeyPress()` → Execute key press via tmux

## Build

- `make build-local`: Keyboard input for development/testing
- `make build-sdl`: Build for machine with SDL input
- `make docker-build`: Build for R36S using Docker (cross-compile)
- `make rebuild-r36s`: Build and deploy to R36S via scp

## Logging

Logs are written to `/tmp/rex.log` with debug-level output.

## Design Decisions

- **R2**: Use this to open/close the keyboard app
- **Synchronous connections**: Only one keyboard allowed at a time
- **Tmux integration**: All key presses are executed via tmux send-keys to pane 0

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [go-sdl2](https://github.com/veandco/go-sdl2) - SDL2 bindings (for R36S build)
- [eiannone/keyboard](https://github.com/eiannone/keyboard) - Keyboard input (for local build)

# TODO

## Bug Reports

- [ ] [JSON decoder out of sync - data changing underfoot?](notes/bugs/json_decode_bug.md)
