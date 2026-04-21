---
name: rex-plugin
description: Write a JavaScript plugin (operator) that maps controller buttons to tmux key strings for a specific tmux window. Use when the user asks to add/edit rex operators, per-app key mappings, or plugins for tools like ncmpcpp, lazygit, lf, etc.
---

# Rex Plugins (JS Operators)

Rex runs one JS plugin per tmux window. The plugin is a pure function:
button code in, tmux key string out. Rex handles `tmux send-keys` itself.

Design background: `notes/features/js-plugins.md`.
Go runtime: `src/server/operator_script.go`, `src/server/plugins.go`.

## File layout

- User plugins: `~/.config/rex/plugins/<window-name>.js`
- Embedded defaults: `src/server/plugins/*.js` (bundled into the binary via
  `//go:embed`; loaded if no user file of that name exists)
- `<window-name>` must match the tmux window name exactly. Rex calls
  `tmux display-message -p '#{window_name}'` to pick the active plugin.
- Override path for dev/tests: set `REX_PLUGIN_DIR` to load from elsewhere.

## Contract

Export a global `handleKey(key)` function. Scripts run as plain globals
(not ES modules) — declare `function handleKey(...)` at top level or
assign to `globalThis.handleKey`.

```js
function handleKey(key) {
  // return a string → rex runs: tmux send-keys -t 0 <string>
  // return null/undefined → rex falls through to the global operator
}
```

- **Input:** `key` is an integer matching one of the injected constants below.
- **Return string:** passed verbatim to `tmux send-keys -t 0`. Can be a
  single character (`"p"`, `">"`), a tmux key name (`"Enter"`, `"Escape"`,
  `"Up"`, `"BSpace"`, `"Space"`), or a chord (`"C-c"`, `"M-x"`).
- **Return `null` / `undefined` / `""`:** not handled — rex falls through
  to the global operator (see below).
- **Throw:** logged and treated as unhandled. A buggy plugin cannot crash rex.
- **Key-up events are ignored.** `handleKey` is only invoked on key-down.
  Do not try to implement press-and-hold semantics here.

## Injected globals (key codes)

These match `protocol.Key*` in `src/protocol/protocol.go`:

```
KEY_DPAD_UP    KEY_DPAD_DOWN    KEY_DPAD_LEFT    KEY_DPAD_RIGHT
KEY_A          KEY_B            KEY_X            KEY_Y
KEY_L1         KEY_R1           KEY_L2           KEY_R2
KEY_SELECT     KEY_START        KEY_FN
```

## Global fallback (what to leave unmapped)

These keys are handled by `GlobalOperator` when the plugin returns `null`,
so most plugins do not need to map them:

| Key     | Default                                   |
|---------|-------------------------------------------|
| A       | `Enter`                                   |
| B       | `Escape`                                  |
| D-pad   | `Up`/`Down`/`Left`/`Right`                |
| L1 / R1 | previous / next tmux window               |
| L1+R1   | switch pane (hold) / kill pane (quick)    |
| Select  | open the rex keyboard pane                |
| Start   | open the rex menu                         |

Only override one of these if the app needs something different
(e.g. ncmpcpp remaps D-pad left/right to cycle screens).

L1, R1, Select, Start are intercepted *before* the plugin runs — you
cannot override them from JS.

## Minimal example — `menu.js`

```js
function handleKey(key) {
  if (key === KEY_A) return "Enter";
  return null;
}
```

## Stateful example — `ncmpcpp.js`

State persists across calls; the plugin's JS runtime is created once at
server start and lives for the process lifetime.

```js
let screen = 0;

function handleKey(key) {
  switch (key) {
    case KEY_DPAD_RIGHT:
      screen = (screen + 1) % 8;
      return String(screen + 1);
    case KEY_DPAD_LEFT:
      screen = (screen - 1 + 8) % 8;
      return String(screen + 1);
    case KEY_B: return "p";   // pause
    case KEY_X: return ">";   // next track
    case KEY_Y: return "<";   // prev track
  }
  return null;
}
```

See `src/server/plugins/ncmpcpp.js` for the shipped copy.

## Workflow

1. Identify the tmux window name the plugin should target
   (e.g. `lazygit`, `lf`). The plugin filename must match.
2. Read the app's key docs or `--help` to find the literal keys it
   expects. The plugin's job is to translate rex buttons to those keys.
3. Start with a minimal `switch` over controller buttons. Return `null`
   for anything you want the global fallback to handle.
4. Drop the file at `~/.config/rex/plugins/<name>.js` (or set
   `REX_PLUGIN_DIR` during dev) and restart the rex server.
5. If you want the plugin to ship as a default, put it in
   `src/server/plugins/` — `//go:embed` picks it up on the next build.

## Testing

Unit tests live beside the Go code. To test a new plugin without a device,
add a case to `src/server/operator_script_test.go` using
`NewScriptOperator` + `op.evalKey(keyCode)` — see the `ncmpcpp` test for
the pattern. Run with `cd src && go test ./server/ -run ScriptOperator`.

## Things to avoid

- **Don't** call `tmux`, spawn processes, or touch the filesystem from JS.
  Plugins are pure. If you need side effects, write a Go operator instead
  (see `operator_ssh_tmux.go`, `operator_claude.go`).
- **Don't** use ES module syntax (`import`, `export`). Scripts are
  evaluated as plain globals.
- **Don't** rely on `console.log` — there is no console binding. Use the
  Go test harness to debug behavior.
- **Don't** return a multi-key string hoping it will send multiple keys.
  Each call to `handleKey` produces exactly one `tmux send-keys` invocation
  with whatever string you returned; tmux will interpret that as a single
  key name or chord. If you need to send several keys, the current
  contract doesn't support it — extend the return type (see "Phase 3"
  in `notes/features/js-plugins.md`) before adding that use case.
- **Don't** map L1, R1, Select, or Start — rex intercepts them before
  the plugin runs.
