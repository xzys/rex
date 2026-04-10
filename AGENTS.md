# Rex

Chorded keyboard input system for retro handhelds (R36S, TrimUI Brick). Go + Bubble Tea.

## Build

```
make build-local              # local dev (keyboard tag)
make build-docker-r36s        # R36S (sdl tag)
make build-docker-brick       # Brick (sdl+brick+sherpa tags)
```

See `Makefile` for all build/sync/install targets.

## Key Directories

- `src/server/` - Input handling, operators
- `src/keyboard/` - Keyboard models and UI
- `src/protocol/` - Socket protocol, key constants
- `src/menu/` - Start menu
- `ext/` - External deps (TermSP, fonts, brick scripts)

## Docs (read when relevant, not upfront)

- `notes/architecture.md` - System design, data flow
- `notes/plans/` - Active implementation plans
- `notes/features/` - Feature designs
- `notes/bugs/` - Known issues
- `notes/research/` - Device/tech research
- `notes/testing.md` - Testing patterns
