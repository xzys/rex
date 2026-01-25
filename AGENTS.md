# Instructions

## Notes

Project learnings and documentation are in `notes/`. Check relevant files before starting work on a feature.

## Building

Build the app locally before testing:

```bash
make build-local
```

### Building External Programs

Build external programs from the `ext/` directory for R36S:

```bash
make docker-build-ext-<program>
```

For example, to build `lf`:

```bash
make docker-build-ext-lf
```

Built binaries are placed in the `bin/` directory. Each program in `ext/` has its own `Dockerfile` and `build.sh` script that handles cross-compilation for ARM64.

### Building and Syncing to R36S

Build rex for R36S:

```bash
make docker-build-r36s
```

This builds the binary and places it in `bin/rex`.

Sync the built binary to the R36S device:

```bash
make sync-r36s
```

This copies `bin/rex` to `r36s:bin/` via SCP.

## Test Scripts

After making changes, run the relevant test scripts

### Start Menu

Test the start menu and exit after one second.
Verifies that the UI renders correctly.

```bash
./scripts/test_menu.sh
```

