# Instructions

## Documentation

- `README.md` - Overview, usage, key bindings
- `notes/architecture.md` - System design, components, data flow
- `notes/testing.md` - Testing patterns and utilities
- `notes/bugs/` - Known issues and investigations

## Building

Build before testing:

```bash
make build-local
```

See `Makefile` for all build targets.

## Key Directories

- `src/server/` - Input handling and operators
- `src/keyboard/` - Keyboard models and UI
- `src/protocol/` - Socket protocol and key constants
- `src/menu/` - Start menu

## Important Notes

- Always resize the browser viewport to ≤1800px width before taking screenshots with `chrome_devtools_take_screenshot`. Images exceeding 2000px in any dimension will fail.
