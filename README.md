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
