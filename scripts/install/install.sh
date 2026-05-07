#!/bin/bash
# Universal rex installer. Dispatches to install-brick.sh or install-r36s.sh
# based on the TARGET file packaged alongside this script.
#
# Modes:
#   ./install.sh                    Install on the current machine.
#   ./install.sh --remote HOST      Push to ssh HOST and install there.
#                                   HOST should be an ssh alias from ~/.ssh/config
#                                   (e.g. "brick-root", "r36s") with key auth set up.
#
# Pass extra args after the target — they are forwarded to the per-target
# installer (e.g. ./install.sh -- ark   to override the R36S username).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ ! -f "$SCRIPT_DIR/TARGET" ]; then
    echo "error: $SCRIPT_DIR/TARGET missing — run from an extracted rex install tarball" >&2
    exit 1
fi
TARGET=$(cat "$SCRIPT_DIR/TARGET")

usage() {
    cat <<EOF
usage: $0 [--remote HOST] [-- INSTALLER_ARGS...]

  --remote HOST   ssh to HOST and run the installer there.
                  Tarball is streamed over ssh; HOST needs key-based auth.
  -- ARGS         pass remaining args to install-${TARGET}.sh

target: $TARGET
version: $(cat "$SCRIPT_DIR/VERSION" 2>/dev/null || echo unknown)
EOF
}

REMOTE=""
EXTRA_ARGS=()
while [ $# -gt 0 ]; do
    case "$1" in
        --remote) REMOTE="$2"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --) shift; EXTRA_ARGS=("$@"); break ;;
        *) echo "unknown arg: $1" >&2; usage >&2; exit 1 ;;
    esac
done

if [ -n "$REMOTE" ]; then
    echo "==> shipping $TARGET install to $REMOTE"
    base=$(basename "$SCRIPT_DIR")
    remote_dir="/tmp/$base"
    # Stream tarball directly into ssh — avoids needing a temp file on host.
    tar cz -C "$(dirname "$SCRIPT_DIR")" "$base" | \
        ssh "$REMOTE" "rm -rf $remote_dir && mkdir -p /tmp && tar xz -C /tmp"
    if [ ${#EXTRA_ARGS[@]} -gt 0 ]; then
        ssh -t "$REMOTE" "cd $remote_dir && ./install.sh -- ${EXTRA_ARGS[*]}"
    else
        ssh -t "$REMOTE" "cd $remote_dir && ./install.sh"
    fi
    exit 0
fi

case "$TARGET" in
    brick) exec "$SCRIPT_DIR/install-brick.sh" "${EXTRA_ARGS[@]}" ;;
    r36s)  exec "$SCRIPT_DIR/install-r36s.sh" "${EXTRA_ARGS[@]}" ;;
    *) echo "unknown target: $TARGET" >&2; exit 1 ;;
esac
