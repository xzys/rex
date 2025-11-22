# Architecture Overview

## Overview

TR3X is a client-server keyboard input system designed for the R36S handheld gaming device. It enables gamepad/joystick input to be translated into keyboard characters through a chorded keyboard interface. The system consists of two main components: a **server** that handles input device events and a **keyboard client** that provides a visual keyboard interface and translates gamepad inputs into characters.

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
- Accepts keyboard client connections via Unix socket (`/tmp/tr3x.sock`)
- Forwards device input to keyboard client as `KeyInputMessage`
- Receives `PressKeyMessage` from keyboard client and executes key presses

### Keyboard Client (`keyboard/`)
- Visual keyboard interface using Bubble Tea
- Connects to server and receives device input
- Translates gamepad inputs to characters using chorded or simple model
- Sends translated characters back to server as `PressKeyMessage`

### Protocol (`protocol/`)
- Defines key constants (D-Pad, face buttons, modifiers)
- Special keys use non-printable control codes (`\x01`-`\x04`) to avoid collisions
- JSON messages over Unix socket, line-delimited

### Keyboard Models
- **Chorded**: Modifier + trigger combinations (L1/R1/L2 + D-Pad/buttons), supports shift and numbers modes
- **Simple**: Move to key an press A to press

## Data Flow

1. Input device → Server (`HandleKey`) → `KeyInputMessage` → Keyboard client
2. Keyboard client → Model (`HandleKey`) → Character → `PressKeyMessage` → Server
3. Server → `pressKey()` → Execute key press

## Build

- `make build-sdl`: Build for R36S
- `make build-local`: Keyboard input for development/testing

## Design Decisions

- **R2**: Use this to open/close the keyboard app
- **Synchronous connections**: Only one keyboard allowed at a time

# TODO

## Bug Reports

### JSON decoder out of sync - data changing underfoot?

```
Caught goroutine panic:
JSON decoder out of sync - data changing underfoot?

goroutine 12 [running]:
runtime/debug.Stack()
    /opt/homebrew/Cellar/go/1.24.1/libexec/src/runtime/debug/stack.go:26 +0x64
runtime/debug.PrintStack()
    /opt/homebrew/Cellar/go/1.24.1/libexec/src/runtime/debug/stack.go:18 +0x1c
github.com/charmbracelet/bubbletea.(*Program).recoverFromGoPanic(0x140000e03c0, {0x10078fac0, 0x1007d7180})
    /Users/sachin/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.5/tea.go:822 +0xa4
github.com/charmbracelet/bubbletea.(*Program).handleCommands.func1.1.1()
    /Users/sachin/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.5/tea.go:360 +0x40
panic({0x10078fac0?, 0x1007d7180?})
    /opt/homebrew/Cellar/go/1.24.1/libexec/src/runtime/panic.go:792 +0x124
encoding/json.(*decodeState).value(0x140000e0528, {0x10078d640?, 0x14000128018?, 0x1006884ac?})
    /opt/homebrew/Cellar/go/1.24.1/libexec/src/encoding/json/decode.go:361 +0x158
encoding/json.(*decodeState).unmarshal(0x140000e0528, {0x10078d640?, 0x14000128018?})
    /opt/homebrew/Cellar/go/1.24.1/libexec/src/encoding/json/decode.go:178 +0x100
encoding/json.(*Decoder).Decode(0x140000e0500, {0x10078d640, 0x14000128018})
    /opt/homebrew/Cellar/go/1.24.1/libexec/src/encoding/json/stream.go:73 +0x118
github.com/xzys/TR3X/v2/protocol.ParseMessage(0x140000e0500)
    /Users/sachin/drive/projects/r36s/protocol/protocol.go:69 +0x44
github.com/xzys/TR3X/v2/keyboard.(*Keyboard).ReadSocket(0x2?)
    /Users/sachin/drive/projects/r36s/keyboard/keyboard.go:94 +0x20
github.com/charmbracelet/bubbletea.(*Program).handleCommands.func1.1()
    /Users/sachin/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.5/tea.go:365 +0x64
created by github.com/charmbracelet/bubbletea.(*Program).handleCommands.func1 in goroutine 27
    /Users/sachin/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.5/tea.go:355 +0x10c
```

The panic `JSON decoder out of sync - data changing underfoot?` occurs due to **concurrent access to a `json.Decoder`**. The `json.Decoder` type is not safe for concurrent use.

**Root Cause:**
- `ReadSocket()` is a Bubble Tea command (`tea.Cmd`) that can execute in goroutines
- Multiple `ReadSocket()` calls can run concurrently (from `Init()`, `Update()` via `tea.Sequence()`, and other message handlers)
- All share the same `k.decoder` instance created in `Init()`
- When multiple goroutines call `decoder.Decode()` simultaneously, the decoder's internal state becomes corrupted, causing the panic

**Solution:**
Serialize decoder access so only one goroutine reads at a time. Options:
1. **Mutex** - Simplest approach, wrap decoder access with `sync.Mutex`
2. **Channel-based serialization** - Use a dedicated reader goroutine with channels
3. **Restructure** - Avoid concurrent calls (more complex)
