#!/usr/bin/env bash
# Install KittenTTS runtime dependencies on TrimUI Brick (Debian chroot).
#
# The kitten-tts binary is built by `make build-docker-kitten-tts` and deployed
# by `sync-brick` / `install-brick.sh`. This script sets up runtime deps:
#   - official onnxruntime v1.23.2 library
#   - espeak-ng for phoneme generation
#   - TTS models (nano or mini)
#
# Run inside the Debian chroot as root (for apt/dpkg) or rex user (if deps
# are already installed). Idempotent.

set -euo pipefail

ORT_VERSION="${ORT_VERSION:-1.23.2}"
ORT_URL="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-aarch64-${ORT_VERSION}.tgz"
ORT_LIB="/usr/local/lib/libonnxruntime.so.${ORT_VERSION}"

BIN_DIR="${KITTEN_TTS_BIN_DIR:-$HOME/bin}"
MODELS_DIR="${KITTEN_TTS_MODELS_DIR:-$HOME/models}"
MODEL_NAME="${KITTEN_TTS_MODEL_NAME:-kitten-tts-nano-int8}"
MODEL_URL="https://huggingface.co/KittenML/kitten-tts-nano-0.8-int8/resolve/main"

command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }

# --- 1. install onnxruntime library ----------------------------------------
if [ ! -f "$ORT_LIB" ]; then
    echo "==> Installing onnxruntime ${ORT_VERSION}..."
    work="$(mktemp -d)"
    cleanup() { rm -rf "$work"; }
    trap cleanup EXIT
    curl -fL "$ORT_URL" -o "$work/ort.tgz"
    tar xzf "$work/ort.tgz" -C "$work"
    cp "$work/onnxruntime-linux-aarch64-${ORT_VERSION}/lib/libonnxruntime.so.${ORT_VERSION}" /tmp/
    chmod 755 "/tmp/libonnxruntime.so.${ORT_VERSION}"
    if [ "$(id -u)" -eq 0 ]; then
        mv "/tmp/libonnxruntime.so.${ORT_VERSION}" "$ORT_LIB"
        ln -sf "$ORT_LIB" /usr/local/lib/libonnxruntime.so
        ln -sf "$ORT_LIB" /usr/local/lib/libonnxruntime.so.1
        ldconfig
    else
        echo "warning: not root — install onnxruntime manually:" >&2
        echo "  sudo cp /tmp/libonnxruntime.so.${ORT_VERSION} /usr/local/lib/ && sudo ldconfig" >&2
        exit 1
    fi
    trap - EXIT
    rm -rf "$work"
else
    echo "onnxruntime ${ORT_VERSION} already installed at $ORT_LIB"
fi

# --- 2. install espeak-ng --------------------------------------------------
if ! command -v espeak-ng >/dev/null 2>&1; then
    echo "==> Installing espeak-ng..."
    if [ "$(id -u)" -eq 0 ]; then
        apt-get update -qq && apt-get install -y -qq espeak-ng
    else
        echo "warning: not root — install espeak-ng manually:" >&2
        echo "  sudo apt-get install -y espeak-ng" >&2
        exit 1
    fi
else
    echo "espeak-ng already installed"
fi

# --- 3. download TTS model -------------------------------------------------
mkdir -p "$MODELS_DIR/$MODEL_NAME"

model_files=(
    "config.json"
    "kitten_tts_nano_v0_8.onnx"
    "voices.npz"
)

missing=0
for f in "${model_files[@]}"; do
    if [ ! -f "$MODELS_DIR/$MODEL_NAME/$f" ]; then
        echo "==> Downloading model file: $f..."
        curl -fL "$MODEL_URL/$f" -o "$MODELS_DIR/$MODEL_NAME/$f"
    fi
done

chmod -R a+rX "$MODELS_DIR/$MODEL_NAME"
echo "Model installed: $MODELS_DIR/$MODEL_NAME"

# --- 4. set up LD_LIBRARY_PATH in profile -----------------------------------
profile_line='export LD_LIBRARY_PATH=/usr/local/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}'
if ! grep -q "LD_LIBRARY_PATH.*onnx" "$HOME/.profile" 2>/dev/null; then
    echo "$profile_line" >> "$HOME/.profile"
fi

cat <<EOF

KittenTTS runtime ready.

Binary:  $BIN_DIR/kitten-tts
Model:   $MODELS_DIR/$MODEL_NAME

Try:
  export LD_LIBRARY_PATH=/usr/local/lib
  kitten-tts $MODELS_DIR/$MODEL_NAME "" --list-voices
  kitten-speak "Hello from Rex on the Brick"
EOF
