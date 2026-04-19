# REX

REX is a client-server keyboard input system designed for the R36S handheld gaming device. It enables gamepad/joystick input to be translated into keyboard characters through a chorded keyboard interface.

## Usage

```
rex server                    # Start the daemon
rex keyboard [--mode MODE]    # Start keyboard (mode: simple, chorded, chorded-simultaneous)
rex menu [--ssh]              # Start menu
rex settings                  # Open settings UI
rex exclusive <path>          # Run app with exclusive SDL access (kills/restarts server)
```

Config file search order: `REX_CONFIG_PATH` → `~/.config/rex/config.yaml` → `$PWD/config.yaml`

Override config path per command:

```bash
REX_CONFIG_PATH=/tmp/rex-config.yaml rex settings
```

### Menu Config

```yaml
keyboard_mode: chorded        # simple | chorded | chorded-simultaneous
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

## Key Bindings

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

### Development Keyboard Mappings

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

## Build

```bash
make build-local              # Build for local development
make docker-build-r36s        # Build for R36S (cross-compile)
make sync-r36s                # Sync binary and config to R36S
make docker-build-ext-<name>  # Build external program from ext/
make install-system-files     # Install dotfiles and systemd services
```

## Documentation

- `notes/architecture.md` - System design, components, data flow
- `notes/protocol.md` - Socket protocol specification
- `notes/keyboard-modes.md` - Chorded and simple keyboard mappings
- `notes/testing.md` - Testing patterns and utilities
- `notes/bugs/` - Known issues and investigations

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [go-sdl2](https://github.com/veandco/go-sdl2) - SDL2 bindings (for R36S build)

## TODO


- input
  - keyboard
    - [ ] adjust keybinds for be efficient
  - voice
    - [X] test companion app on R36S
    - [X] companion app bluetooth trigger to connect ws / send text
- output
  - [ ] 

- abilities
  - code
    - [X] running pi on device
    - [X] running claudde on device
    - [X] claude keymaps
    - [X] run debian inside knulli
    - [ ] pi keymaps
    - [ ] pi kitten tts
    - [ ] pi markdown notes skill
  - remote
    - [ ] reconnect popup
    - [ ] TEST audio relay, install popup
  - music
    - [ ] daw
    - [ ] easy syncing to peers

- usability
  - [ ] rex help screen with controls shown over keyboard, tab for different apps
  - [ ] wifi settings tui
  - [ ] bluetooth settings tui
  - [ ] package to make it easy to install

- publishing
  - [ ] terminal intro, REX stylized text or something
  - [ ] colorize text in 
