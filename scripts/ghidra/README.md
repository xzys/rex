# Ghidra helper scripts

Headless Ghidra post-scripts for the Brick firmware work.
Host setup, memory limits and the raw-binary import flags: `notes/ghidra-runbook.md`.
Why we are doing this at all: `notes/ghidra-linux-port-plan.md`.

```bash
. scripts/ghidra/env.sh
"$GHIDRA_HEADLESS" /tmp/proj brick -import <binary> \
    -scriptPath "$PWD/scripts/ghidra" -postScript <Script>.java -deleteProject
```

| Script | Purpose | Status |
|---|---|---|
| `ListFunctions.java` | dump functions + language + image base | verified on this host |
| `DecompileOne.java` | decompile `add`/`main`; proves the native decompiler works | verified on this host |
| `FindPanelInit.java` | find an Allwinner BSP LCD panel struct by name and its xrefs | **untested** — no firmware on this host yet |

`FindPanelInit.java` is a starting point, not a finished tool. It has never been run
against a real blob because no Brick firmware image is on this machine yet.
