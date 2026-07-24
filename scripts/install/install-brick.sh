#!/bin/bash
# rex installer for the TrimUI Brick (Knulli + Debian chroot).
#
# Run on the device:   ./install-brick.sh
# Run from a host:     ./install.sh --remote brick-root   (calls this over ssh)
#
# Idempotent: existing config files in /userdata/debian/home/rex/.config/rex
# and the Debian rootfs at /userdata/debian are preserved when already present.
# Re-runs reinstall app files, scripts, and (if needed) chroot packages.

set -euo pipefail

# --- where am I -------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PAYLOAD="$SCRIPT_DIR/payload"

if [ ! -d "$PAYLOAD/apps-rex" ]; then
    echo "error: $PAYLOAD/apps-rex not found — run from extracted tarball" >&2
    exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
    echo "error: must run as root (try: sudo $0)" >&2
    exit 1
fi

if [ ! -d /userdata ]; then
    echo "error: /userdata missing — this script targets Knulli on TrimUI Brick" >&2
    exit 1
fi

# Chroot sources, in order of preference:
#   1. Bundled at /boot/rex-bundle/brick-debian.tar.gz (on BATOCERA FAT32,
#      extracted directly — no redundant copy needed).
#   2. Bundled at /userdata/rex-bundle/brick-debian.tar.gz (legacy path,
#      from older images that copied it to SHARE).
#   3. REX_CHROOT_URL env (fallback for installs without bundled image).
#   4. Existing /userdata/debian if /etc/debian_version is present.
BOOT_CHROOT_BUNDLE=/boot/rex-bundle/brick-debian.tar.gz
SHARE_CHROOT_BUNDLE=/userdata/rex-bundle/brick-debian.tar.gz
if [ -f "$BOOT_CHROOT_BUNDLE" ]; then
    CHROOT_BUNDLE="$BOOT_CHROOT_BUNDLE"
else
    CHROOT_BUNDLE="$SHARE_CHROOT_BUNDLE"
fi
CHROOT_URL="${REX_CHROOT_URL:-}"

APP=/userdata/system/apps/rex
TOOLS=/userdata/roms/tools
DEBIAN_ROOT=/userdata/debian
REX_HOME="$DEBIAN_ROOT/home/rex"

# --- 1. install Knulli-side files ------------------------------------------
echo "==> installing $APP"
mkdir -p "$APP/lib" "$APP/fonts"
cp -f "$PAYLOAD/apps-rex/termsp" "$APP/termsp"
cp -f "$PAYLOAD/apps-rex/rex-wifid" "$APP/rex-wifid"
cp -f "$PAYLOAD/apps-rex/lib/"*.so "$APP/lib/"
cp -f "$PAYLOAD/apps-rex/fonts/"*.ttf "$APP/fonts/"
cp -f "$PAYLOAD/apps-rex/"*.sh "$APP/"
cp -f "$PAYLOAD/apps-rex/config.yaml.brick.example" "$APP/config.yaml.brick.example"
ln -sf libvterm.so "$APP/lib/libvterm.so.0"
chmod +x "$APP/termsp" "$APP/rex-wifid" "$APP"/*.sh

# Install STT model files so voice dictation works.
for model_dir in stt stt-parakeet; do
    if [ -d "$PAYLOAD/apps-rex/$model_dir" ] && [ -n "$(ls -A "$PAYLOAD/apps-rex/$model_dir" 2>/dev/null)" ]; then
        echo "==> installing STT model: $model_dir"
        mkdir -p "$APP/$model_dir"
        cp -f "$PAYLOAD/apps-rex/$model_dir/"* "$APP/$model_dir/"
        chmod -R a+rX "$APP/$model_dir"
    fi
done

echo "==> installing $TOOLS/Rex.sh (ES menu launcher)"
mkdir -p "$TOOLS"
cp -f "$PAYLOAD/roms-tools/Rex.sh" "$TOOLS/Rex.sh"
chmod +x "$TOOLS/Rex.sh"

# --- 2. ensure Debian chroot exists -----------------------------------------
if [ ! -e "$DEBIAN_ROOT/etc/debian_version" ]; then
    mkdir -p "$DEBIAN_ROOT"
    if [ -n "$(ls -A "$DEBIAN_ROOT" 2>/dev/null)" ]; then
        echo "error: $DEBIAN_ROOT exists and is non-empty but missing /etc/debian_version" >&2
        echo "       refusing to overwrite — inspect manually" >&2
        exit 1
    fi

    if [ -f "$CHROOT_BUNDLE" ]; then
        echo "==> extracting bundled Debian rootfs from $CHROOT_BUNDLE"
        tar xzf "$CHROOT_BUNDLE" -C "$DEBIAN_ROOT"
    elif [ -n "$CHROOT_URL" ]; then
        echo "==> downloading Debian rootfs from $CHROOT_URL"
        tmp=$(mktemp -p /userdata)
        curl -fL --progress-bar -o "$tmp" "$CHROOT_URL"
        tar xzf "$tmp" -C "$DEBIAN_ROOT"
        rm -f "$tmp"
    else
        cat >&2 <<EOF
error: Debian chroot not found at $DEBIAN_ROOT.

Expected one of:
  - bundled rootfs at /boot/rex-bundle/ or /userdata/rex-bundle/
  - REX_CHROOT_URL set to a Debian arm64 rootfs tarball (.tar.gz)
  - existing /userdata/debian/ (manually staged)
EOF
        exit 1
    fi
fi

# --- 3. drop chroot helper scripts and rex binary into the chroot -----------
echo "==> installing rex binary into chroot at /home/rex/bin/rex"
install -d -m 755 "$REX_HOME/bin" "$REX_HOME/.config/rex"
cp -f "$PAYLOAD/home-rex/bin/rex" "$REX_HOME/bin/rex"
if [ -f "$PAYLOAD/home-rex/bin/kitten-speak" ]; then
    cp -f "$PAYLOAD/home-rex/bin/kitten-speak" "$REX_HOME/bin/kitten-speak"
fi
cp -f "$PAYLOAD/home-rex/.tmux.conf" "$REX_HOME/.tmux.conf"
cp -f "$PAYLOAD/home-rex/.tmux-wifi.sh" "$REX_HOME/.tmux-wifi.sh"
chmod +x "$REX_HOME/.tmux-wifi.sh"

# Seed user config from example only if no config exists yet.
if [ ! -f "$REX_HOME/.config/rex/config.yaml" ]; then
    cp "$APP/config.yaml.brick.example" "$REX_HOME/.config/rex/config.yaml"
fi

chmod +x "$REX_HOME/bin/rex"
[ -f "$REX_HOME/bin/kitten-speak" ] && chmod +x "$REX_HOME/bin/kitten-speak"

# Copy sherpa libs into chroot /usr/local/lib so rex resolves them at runtime.
# Always run ldconfig afterward — the chroot setup block may be skipped for
# pre-baked chroots, but the shared lib cache must be refreshed.
for lib in libsherpa-onnx-c-api.so libonnxruntime.so; do
    if [ -f "$APP/lib/$lib" ]; then
        cp -u "$APP/lib/$lib" "$DEBIAN_ROOT/usr/local/lib/"
    fi
done
"$APP/debian-enter.sh" --root ldconfig

# --- 4. chroot enter helper -------------------------------------------------
chroot_run_root() {
    "$APP/debian-enter.sh" --root "$@"
}

# --- 5. one-time chroot setup (idempotent) ---------------------------------
needs_setup=0
if [ ! -f "$DEBIAN_ROOT/etc/sudoers.d/rex-nopasswd" ]; then needs_setup=1; fi
if ! grep -q "^en_US.UTF-8" "$DEBIAN_ROOT/etc/locale.gen" 2>/dev/null; then needs_setup=1; fi
if [ ! -x "$DEBIAN_ROOT/usr/bin/tmux" ]; then needs_setup=1; fi

if [ "$needs_setup" = 1 ]; then
    echo "==> running first-time chroot setup (apt, locale, rex user, sshd)"
    echo "    this takes several minutes on first run (skipped with pre-baked chroot)"

    chroot_run_root "ldconfig"

    # bookworm-backports for tmux 3.5a (extended-keys-format)
    if [ ! -f "$DEBIAN_ROOT/etc/apt/sources.list.d/backports.list" ]; then
        chroot_run_root "echo deb http://deb.debian.org/debian bookworm-backports main > /etc/apt/sources.list.d/backports.list"
    fi

    # NodeSource for Node 20 (bookworm ships 18, lacks ES2024 regex-v)
    chroot_run_root "
        apt update -qq &&
        apt install -y -qq gnupg ca-certificates curl &&
        [ -f /usr/share/keyrings/nodesource.gpg ] || {
            curl -fsSL -o /tmp/nodesource.key https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key &&
            gpg --dearmor --yes -o /usr/share/keyrings/nodesource.gpg /tmp/nodesource.key &&
            rm -f /tmp/nodesource.key;
        } &&
        [ -f /etc/apt/sources.list.d/nodesource.list ] ||
            echo 'deb [signed-by=/usr/share/keyrings/nodesource.gpg] https://deb.nodesource.com/node_20.x nodistro main' > /etc/apt/sources.list.d/nodesource.list
    "

    chroot_run_root "
        apt update -qq &&
        apt install -y -qq sudo libsdl2-2.0-0 openssh-server locales alsa-utils nodejs &&
        apt install -y -qq -t bookworm-backports tmux
    "

    chroot_run_root "
        grep -q ^en_US.UTF-8 /etc/locale.gen || echo 'en_US.UTF-8 UTF-8' >> /etc/locale.gen &&
        locale-gen en_US.UTF-8 >/dev/null &&
        update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8
    "

    chroot_run_root "
        id rex >/dev/null 2>&1 || useradd -m -s /bin/bash -G sudo,audio,video rex &&
        usermod -aG sudo,audio,video rex &&
        getent group inet >/dev/null 2>&1 || groupadd -g 3003 inet &&
        usermod -aG inet rex &&
        echo 'rex ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/rex-nopasswd &&
        chmod 440 /etc/sudoers.d/rex-nopasswd
    "

    # sshd config: port 2222, copy authorized_keys from Knulli host if present
    if [ -f /userdata/system/.ssh/authorized_keys ]; then
        chroot_run_root "
            mkdir -p /etc/ssh/sshd_config.d &&
            echo 'Port 2222' > /etc/ssh/sshd_config.d/rex.conf &&
            echo 'PermitRootLogin prohibit-password' >> /etc/ssh/sshd_config.d/rex.conf &&
            mkdir -p /root/.ssh /home/rex/.ssh &&
            chmod 700 /root/.ssh /home/rex/.ssh &&
            cp /userdata/system/.ssh/authorized_keys /root/.ssh/authorized_keys &&
            cp /userdata/system/.ssh/authorized_keys /home/rex/.ssh/authorized_keys &&
            chmod 600 /root/.ssh/authorized_keys /home/rex/.ssh/authorized_keys
        "
    fi
fi

# Always re-chown — rex binary copied as root via cp -f
chroot_run_root "chown -R rex:rex /home/rex"

# --- 6. done ---------------------------------------------------------------

echo ""
echo "rex installed."
echo "  launch over ssh from host:     ssh brick-root '$APP/start-rex.sh'"
