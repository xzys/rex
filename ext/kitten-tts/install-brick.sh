#!/usr/bin/env bash
# Install second-state/kitten_tts_rs for Rex on TrimUI Brick.
# Run inside the Debian chroot as the rex user, or from a workstation with:
#   make install-kitten-tts-brick

set -euo pipefail

VERSION="${KITTEN_TTS_RS_VERSION:-v0.2.2}"
BASE_URL="${KITTEN_TTS_RS_BASE_URL:-https://github.com/second-state/kitten_tts_rs/releases/download/$VERSION}"
BIN_DIR="${KITTEN_TTS_BIN_DIR:-$HOME/bin}"
MODELS_DIR="${KITTEN_TTS_MODELS_DIR:-$HOME/models}"
DEFAULT_MODEL_NAME="${KITTEN_TTS_DEFAULT_MODEL_NAME:-kitten-tts-mini}"

case "$(uname -m)" in
    aarch64|arm64)
        ASSET_ARCH="aarch64-linux"
        ;;
    x86_64|amd64)
        ASSET_ARCH="x86_64-linux"
        ;;
    *)
        echo "unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo "tar is required" >&2; exit 1; }

work="$(mktemp -d)"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

bin_url="$BASE_URL/kitten-tts-$ASSET_ARCH.tar.gz"
models_url="$BASE_URL/kitten-tts-models.tar.gz"

mkdir -p "$BIN_DIR" "$MODELS_DIR"

echo "Downloading kitten_tts_rs $VERSION ($ASSET_ARCH)..."
curl -fL "$bin_url" -o "$work/kitten-tts-bin.tar.gz"

echo "Downloading KittenTTS models..."
curl -fL "$models_url" -o "$work/kitten-tts-models.tar.gz"

mkdir -p "$work/bin" "$work/models"
tar xzf "$work/kitten-tts-bin.tar.gz" -C "$work/bin"
tar xzf "$work/kitten-tts-models.tar.gz" -C "$work/models"

kitten_bin="$(find "$work/bin" -type f -name kitten-tts -perm -111 | head -n 1 || true)"
kitten_server="$(find "$work/bin" -type f -name kitten-tts-server -perm -111 | head -n 1 || true)"

if [ -z "$kitten_bin" ]; then
    kitten_bin="$(find "$work/bin" -type f -name kitten-tts | head -n 1 || true)"
fi
if [ -z "$kitten_server" ]; then
    kitten_server="$(find "$work/bin" -type f -name kitten-tts-server | head -n 1 || true)"
fi
if [ -z "$kitten_bin" ]; then
    echo "kitten-tts binary not found in release archive" >&2
    exit 1
fi

install -m 0755 "$kitten_bin" "$BIN_DIR/kitten-tts"
if [ -n "$kitten_server" ]; then
    install -m 0755 "$kitten_server" "$BIN_DIR/kitten-tts-server"
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$script_dir/kitten-speak" ]; then
    install -m 0755 "$script_dir/kitten-speak" "$BIN_DIR/kitten-speak"
else
    echo "warning: kitten-speak wrapper not found next to installer; skipping wrapper install" >&2
fi

preferred_model="$(find "$work/models" -type d -name "$DEFAULT_MODEL_NAME" | head -n 1 || true)"
if [ -z "$preferred_model" ]; then
    preferred_model="$(find "$work/models" -type d -name 'kitten-tts-*' | sort | head -n 1 || true)"
fi
if [ -z "$preferred_model" ]; then
    echo "no kitten-tts-* model directory found in models archive" >&2
    exit 1
fi

model_name="$(basename "$preferred_model")"
rm -rf "$MODELS_DIR/$model_name"
cp -a "$preferred_model" "$MODELS_DIR/"

cat <<EOF

KittenTTS installed.

Binaries:
  $BIN_DIR/kitten-tts
  $BIN_DIR/kitten-tts-server
  $BIN_DIR/kitten-speak

Model:
  $MODELS_DIR/$model_name

Try:
  export PATH="$BIN_DIR:\$PATH"
  kitten-speak --list-voices
  kitten-speak "Hello from Rex on the Brick"
EOF
