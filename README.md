# Architecture Overview

## Overview

REX is a client-server keyboard input system designed for the R36S handheld gaming device. It enables gamepad/joystick input to be translated into keyboard characters through a chorded keyboard interface. The system consists of two main components: a **server** that handles input device events and a **keyboard client** that provides a visual keyboard interface and translates gamepad inputs into characters.

## Usage

```
rex server                    # Start the daemon
rex keyboard [--mode MODE]    # Start keyboard (mode: chord or simple)
rex menu [--config PATH]      # Start menu
rex exclusive <path>          # Run app with exclusive SDL access (kills/restarts server)
```

Config file search order for menu: `--config PATH` → `~/.config/rex/config.yaml` → `$PWD/config.yaml`

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
- Config structure:
  ```yaml
  layout:
    gap: 4           # spacing between items
    margin_x: 4      # horizontal margin
    margin_y: 2      # vertical margin
    square_size: 9   # item size
  apps:
    - name: app_name
      icon: "√"               # single character icon
      color: 1                # ANSI color code
      executable_path: /path  # or just binary name
      exclusive: true         # optional: kills rex server while running
  ```

### Protocol (`protocol/`)
- Defines key constants (D-Pad, face buttons, modifiers)
- Special keys use non-printable control codes (`\x01`-`\x04`) to avoid collisions
- JSON messages over Unix socket, line-delimited

### Operators (`server/operator*.go`)
- Handle app-specific key bindings when keyboard is not active
- `GlobalOperator`: Manages tmux windows and routes keys to active operator
- `MenuOperator`: A=Enter
- `NcmpcppOperator`: A=select, B=play/pause, X=skip, Y=back, Left/Right=cycle screens

### Keyboard Models
- **Chorded**: Modifier + trigger combinations (L1/R1/L2 + D-Pad/buttons), supports shift and numbers modes
- **Simple**: Move to key and press A to select

## Global Key Bindings

When keyboard is **not** active:
| Key | Action |
|-----|--------|
| Start | Open menu |
| Select | Open keyboard pane |
| R1 | Next tmux window |
| L1 | Previous tmux window |
| L2 | New tmux window |
| D-Pad | Arrow keys (sent to tmux) |

When keyboard **is** active:
| Key | Action |
|-----|--------|
| Select | Close keyboard pane |
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
| s | Select |
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
- `make docker-build-r36s`: Build for R36S using Docker (cross-compile)
- `make sync-r36s`: Sync binary and config to R36S via rsync
- `make docker-build-ext-<name>`: Build external program from `ext/` (e.g., `make docker-build-ext-lf`)
- `make install-system-files`: Install dotfiles and systemd services to R36S

## Logging

Logs are written to `/tmp/rex.log` with debug-level output.

## Design Decisions

- **Select**: Use this to open/close the keyboard app
- **Synchronous connections**: Only one keyboard allowed at a time
- **Tmux integration**: All key presses are executed via tmux send-keys to pane 0

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework ([docs](https://pkg.go.dev/github.com/charmbracelet/bubbletea))
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling ([docs](https://pkg.go.dev/github.com/charmbracelet/lipgloss))
- [go-sdl2](https://github.com/veandco/go-sdl2) - SDL2 bindings (for R36S build) ([docs](https://pkg.go.dev/github.com/veandco/go-sdl2))

# TODO

## Features

- [ ] bubbletea app to connect to SSH -- the name of the pane


## Bug Reports

- [ ] [JSON decoder out of sync - data changing underfoot?](notes/bugs/json_decode_bug.md)
