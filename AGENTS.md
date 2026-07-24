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
- `src/wifi/` - WiFi: ConnMan D-Bus client (daemon-side), socket RPC client (rex-side), TUI, error classification
- `src/cmd/rex-wifid/` - Host-side WiFi daemon
- `ext/` - External deps (TermSP, fonts, brick scripts)

## Deploy to Brick

```
make build-docker-brick       # builds bin/rex-brick + bin/rex-wifid
make sync-brick                # scp binaries, scripts, fonts to the device
```

Device IP is in `~/.ssh/config` as `brick-root` (host root, port 22) and
`brick` (chroot rex user, port 2222). Password for both is `linux`.

If scp fails with "dest open Failure", the running binary is locked:
```bash
ssh brick-root 'fuser -k /userdata/debian/home/rex/bin/rex; mv /userdata/debian/home/rex/bin/rex /userdata/debian/home/rex/bin/rex.old'
# then make sync-brick again
```

## Architecture (Brick)

```
┌──────────────────────────────────────────────────┐
│  HOST (Knulli / Buildroot)                       │
│                                                  │
│  ┌──────────┐    D-Bus     ┌──────────────┐      │
│  │ ConnMan  │◄─────────────│  rex-wifid   │      │
│  │ (root)   │              │  (root)      │      │
│  └──────────┘              │              │      │
│                            │ /tmp/rex-    │      │
│                            │ wifi.sock    │      │
│                            └──▲───────────┘      │
│                               │                  │
│  ┌───────────────────────────┼────────────────┐  │
│  │  DEBIAN CHROOT            │                │  │
│  │  /tmp bind-mounted from host               │  │
│  │                           │                │  │
│  │  ┌──────────┐  JSON over   │                │  │
│  │  │ rex      │◄─Unix socket─┘                │  │
│  │  │ (rex,    │                                │  │
│  │  │ uid 1000)│                                │  │
│  │  └──────────┘                                │  │
│  └──────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

- **rex-wifid** daemon runs as root on the host, talks to ConnMan over D-Bus
- Rex (chroot) talks to daemon over `/tmp/rex-wifi.sock` (Unix socket, JSON)
- No D-Bus policy files, no uid mapping, no cross-chroot binary execution
- Shutdown and "Games" (ES) use flag files on `/tmp` (bind-mounted):
  `/tmp/.rex-shutdown-request` and `/tmp/.rex-start-es`
- `ext/brick/start-rex.sh` owns the full lifecycle: launches daemon, launches
  termsp, EXIT trap dispatches restart/shutdown/ES based on flag files
- Chroot boundary: host binaries can't run inside chroot (glibc 2.38 vs 2.36
  mismatch). Use the daemon for host capabilities, or stage binaries
  (e.g. `/usr/bin/brightness` → `$APP/brightness`)

## Debugging on the Brick

### Logs

All logs go to `/tmp/rex-launch.log` (host side):

| Source | Format | Example |
|---|---|---|
| Daemon (Go log) | `YYYY/MM/DD HH:MM:SS message` | `2026/07/24 06:37:52 req err-test cmd=set-powered: missing 'on' field` |
| start-rex.sh | `=== rex starting at ... ===` | `=== rex exiting at 2026-07-24T06:37:44+02:00, restarting Rex ===` |
| rex Go runtime | Timestamped to stderr | `2026/07/24 06:37:21 open terminal failed: not a terminal` (goes to rex-launch.log via `2>&1`) |
| rex panics | Stack traces | Written to `/tmp/rex-stderr.log` inside chroot → visible on host at same path (/tmp bind mount) |

```bash
# Watch live
ssh brick-root 'tail -f /tmp/rex-launch.log'
# Daemon entries only (Go log format)
ssh brick-root 'grep -E "^[0-9]{4}/[0-9]{2}/[0-9]{2}" /tmp/rex-launch.log'
# Rex lifecycle only
ssh brick-root 'grep "===" /tmp/rex-launch.log'
# Rex crashes & panics
ssh brick-root 'cat /tmp/rex-stderr.log'
```

### Daemon (rex-wifid)

```bash
# Is it running?
ssh brick-root 'pidof rex-wifid'
# Test it manually (host)
ssh brick-root 'python3 -c "
import socket,json
s=socket.socket(socket.AF_UNIX);s.settimeout(2)
s.connect(\"/tmp/rex-wifi.sock\")
s.sendall(b\"{\\\"id\\\":\"1\",\\\"cmd\\\":\"status\"}\\n\")
print(json.loads(s.recv(4096)))
"'
# Restart it
ssh brick-root 'killall rex-wifid; rm -f /tmp/rex-wifi.sock; /userdata/system/apps/rex/start-rex.sh'
# Check D-Bus (host) — is ConnMan running?
ssh brick-root 'dbus-send --system --print-reply --dest=net.connman / net.connman.Manager.GetTechnologies'
```

### Common issues

| Symptom | Check |
|---|---|
| Wifi toggle flickers back | Tick race with ConnMan async settle; `lastToggle` timestamp in `settings.go` suppresses tick for 3s after toggle |
| Black screen on exit (no restart) | Old daemon still running? `pidof rex-wifid`. Also check `start-rex.sh` EXIT trap has default restart |
| Restart loop (rex exits instantly) | Multiple start-rex.sh calls racing; `pidof termsp` guard prevents double-launch. Check log for "duplicate session" or "not a terminal" |
| "WiFi unavailable" in settings | Daemon can't reach ConnMan. Check `DBUS_SYSTEM_BUS_ADDRESS` in start-rex.sh — Brick uses `/run/dbus/system_bus_socket` not `/var/run/dbus/` |
| Tmux bar shows "off" for wifi | `python3` missing in chroot? `ssh brick '/userdata/system/apps/rex/debian-enter.sh which python3'`. Old `.tmux-wifi.sh` using `iw`? New script uses python3 socket query |
| scp "dest open Failure" | Binary in use; `fuser -k` the path, then `mv` old binary aside, then sync |

### Manual daemon test commands

```bash
ssh brick-root "echo '{\"id\":\"1\",\"cmd\":\"ping\"}' | python3 -c \"
import socket,json,sys
s=socket.socket(socket.AF_UNIX);s.settimeout(2);s.connect('/tmp/rex-wifi.sock')
s.sendall(sys.stdin.buffer.read())
print(json.loads(s.recv(4096)))
\""
```

Valid commands: `ping`, `status`, `powered`, `set-powered` (needs `"on":true/false`),
`scan`, `services`, `connect` (needs `"path":"...", "passphrase":"..."`),
`disconnect`, `remove`, `capture`.
