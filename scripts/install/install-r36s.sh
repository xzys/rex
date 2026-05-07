#!/bin/bash
# rex installer for the R36S (Knulli/ArkOS-style userland, user 'ark').
#
# Run on the device:   ./install-r36s.sh
# Run from a host:     ./install.sh --remote r36s
#
# Idempotent: re-runs replace binary, dotfiles, systemd units, and /opt scripts.
# Existing ~/.config/rex/config.yaml is preserved.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PAYLOAD="$SCRIPT_DIR/payload"

if [ ! -d "$PAYLOAD/bin" ]; then
    echo "error: $PAYLOAD/bin not found — run from extracted tarball" >&2
    exit 1
fi

# Target user: arg override > $REX_USER > $SUDO_USER > 'ark'
TARGET_USER="${1:-${REX_USER:-${SUDO_USER:-ark}}}"
TARGET_HOME=$(getent passwd "$TARGET_USER" | cut -d: -f6)
if [ -z "$TARGET_HOME" ]; then
    echo "error: user '$TARGET_USER' has no home directory" >&2
    exit 1
fi

echo "==> installing for user $TARGET_USER (home: $TARGET_HOME)"

# --- 1. binary --------------------------------------------------------------
install -d -m 755 -o "$TARGET_USER" -g "$TARGET_USER" "$TARGET_HOME/bin"
install -m 755 -o "$TARGET_USER" -g "$TARGET_USER" \
    "$PAYLOAD/bin/rex" "$TARGET_HOME/bin/rex"

# --- 2. config (preserve existing) -----------------------------------------
install -d -m 755 -o "$TARGET_USER" -g "$TARGET_USER" "$TARGET_HOME/.config/rex"
if [ ! -f "$TARGET_HOME/.config/rex/config.yaml" ]; then
    install -m 644 -o "$TARGET_USER" -g "$TARGET_USER" \
        "$PAYLOAD/config/config.yaml" "$TARGET_HOME/.config/rex/config.yaml"
fi

# --- 3. dotfiles ------------------------------------------------------------
echo "==> installing dotfiles"
for src in "$PAYLOAD/dot/"*; do
    name=$(basename "$src")
    dest="$TARGET_HOME/.$name"
    install -m 644 -o "$TARGET_USER" -g "$TARGET_USER" "$src" "$dest"
done

# --- 4. /opt scripts --------------------------------------------------------
if [ -d "$PAYLOAD/opt" ] && [ -n "$(ls -A "$PAYLOAD/opt" 2>/dev/null)" ]; then
    if [ "$(id -u)" -ne 0 ]; then
        echo "warn: skipping /opt install — re-run as root to install /opt scripts" >&2
    else
        echo "==> installing /opt"
        cp -r "$PAYLOAD/opt/." /opt/
    fi
fi

# --- 5. systemd units -------------------------------------------------------
if [ -d "$PAYLOAD/systemd" ] && [ -n "$(ls -A "$PAYLOAD/systemd" 2>/dev/null)" ]; then
    if [ "$(id -u)" -ne 0 ]; then
        echo "warn: skipping systemd install — re-run as root to install services" >&2
    else
        echo "==> installing systemd services"
        for f in "$PAYLOAD/systemd/"*.service; do
            cp "$f" /etc/systemd/system/
        done
        for d in "$PAYLOAD/systemd/"*.service.d; do
            [ -d "$d" ] && cp -r "$d" /etc/systemd/system/
        done
        systemctl daemon-reload
    fi
fi

echo ""
echo "rex installed for $TARGET_USER."
echo "  enable on boot:  sudo systemctl enable rex.service"
echo "  start now:       sudo systemctl start rex.service"
