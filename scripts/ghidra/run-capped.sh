#!/usr/bin/env bash
# Run Ghidra headless under a hard, kernel-enforced memory cap -- no root, no Docker.
#
# Why: this box is 3.8GB. Ghidra's default 2G heap plus JVM non-heap overhead can push
# the machine into swap and take the whole session down. GHIDRA_HEADLESS_MAXMEM caps the
# *heap*; this caps the whole process tree via a cgroup v2 scope, which is the part
# GHIDRA_HEADLESS_MAXMEM cannot do.
#
# Verified on this host: a 200M cap OOM-kills the JVM, so the limit is real and enforced.
#
#   ./scripts/ghidra/run-capped.sh <proj_dir> <proj_name> -import <binary> [...]
#
# Tunables:
#   GHIDRA_MEM_MAX   total process-tree cap (default 1600M)
#   GHIDRA_HEADLESS_MAXMEM  JVM heap, must stay well under the above (default 1200M)
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$HERE/env.sh"

MEM_MAX="${GHIDRA_MEM_MAX:-1600M}"

# Ghidra requires the project directory to already exist; it will not create it.
if [ $# -ge 1 ] && [ "${1#-}" = "$1" ]; then
    mkdir -p "$1"
fi

exec systemd-run --user --scope -q \
    -p MemoryMax="$MEM_MAX" \
    -p MemorySwapMax=0 \
    --unit="ghidra-$$" \
    "$GHIDRA_HEADLESS" "$@"
