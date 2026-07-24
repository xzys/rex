#!/usr/bin/env bash
# Build kitten-tts for TrimUI Brick (aarch64, Debian Bookworm).
# Runs inside the Docker container; outputs to /output/kitten-tts.
set -euo pipefail

KITTEN_TTS_SRC="${KITTEN_TTS_SRC:-https://github.com/second-state/kitten_tts_rs.git}"
KITTEN_TTS_REF="${KITTEN_TTS_REF:-v0.2.1}"

echo "==> Cloning kitten_tts_rs ${KITTEN_TTS_REF}..."
git clone --depth 1 --branch "$KITTEN_TTS_REF" "$KITTEN_TTS_SRC" /tmp/kitten-tts-src

cd /tmp/kitten-tts-src

# Override ort dependency to disable default features (especially api-24 which
# requires onnxruntime 1.24+ — we pin to 1.23.2 for Bookworm glibc compat).
sed -i 's/ort = "2.0.0-rc.12"/ort = { version = "2.0.0-rc.12", default-features = false }/' Cargo.toml

echo "==> Building kitten-tts..."
cargo build --release \
    --features "ort/std,ort/ndarray,ort/tracing,ort/tls-native,ort/copy-dylibs,ort/api-23"

mkdir -p /output
cp target/release/kitten-tts /output/kitten-tts

echo "==> Built: $(file /output/kitten-tts)"
echo "    Size:  $(du -h /output/kitten-tts | cut -f1)"
