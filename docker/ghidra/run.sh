#!/usr/bin/env bash
# Convenience wrapper for the containerised Ghidra headless analyser.
# Native fallback and memory rationale: notes/ghidra-runbook.md
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE="${GHIDRA_IMAGE:-ghidra-headless:12.1.3}"
PROJ="${GHIDRA_PROJECTS:-$REPO/../ghidra-projects}"
mkdir -p "$PROJ"

exec docker run --rm \
    --memory=1600m --memory-swap=1600m --cpus=3 \
    -v "$REPO/targets:/work/targets" \
    -v "$REPO/scripts/ghidra:/work/scripts:ro" \
    -v "$PROJ:/work/proj" \
    "$IMAGE" "$@"
