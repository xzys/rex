#!/usr/bin/env bash
# Build distributable install tarballs for rex.
#
# Usage:
#   scripts/package.sh                 # both targets
#   scripts/package.sh brick           # just brick
#   scripts/package.sh r36s            # just r36s
#
# Output:
#   dist/rex-brick-<version>.tar.gz
#   dist/rex-r36s-<version>.tar.gz
#
# Each tarball is self-contained: extract, run ./install.sh on-device or
# ./install.sh --remote <ssh-host> from a workstation.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST_DIR="$REPO_ROOT/dist"
INSTALL_DIR="$REPO_ROOT/scripts/install"

# Version: short git sha unless VERSION env override is set. Tag-based versions
# would be cleaner once we cut releases, but sha is fine for now.
VERSION="${VERSION:-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)}"

TARGETS=("$@")
if [ ${#TARGETS[@]} -eq 0 ]; then
    TARGETS=(brick r36s)
fi

mkdir -p "$DIST_DIR"

require_file() {
    local path="$1" hint="$2"
    if [ ! -f "$path" ]; then
        echo "Missing: $path" >&2
        echo "Hint: $hint" >&2
        exit 1
    fi
}

stage_brick() {
    local stage="$1"
    local app="$stage/payload/apps-rex"
    local tools="$stage/payload/roms-tools"
    local home="$stage/payload/home-rex"

    echo "==> staging brick payload"
    mkdir -p "$app/lib" "$app/fonts" "$tools" "$home/bin" "$home/.config/rex"

    require_file "$REPO_ROOT/bin/rex-brick" "make build-docker-brick"
    require_file "$REPO_ROOT/bin/rex-wifid" "make build-docker-brick"
    require_file "$REPO_ROOT/bin/termsp" "make build-docker-termsp"
    require_file "$REPO_ROOT/ext/TermSP/TermSP/libs/libsdlfox.so" \
        "git submodule update --init ext/TermSP/TermSP"
    require_file "$REPO_ROOT/ext/TermSP/TermSP/libs/libvterm.so" \
        "git submodule update --init ext/TermSP/TermSP"

    cp "$REPO_ROOT/bin/termsp" "$app/termsp"
    cp "$REPO_ROOT/bin/rex-wifid" "$app/rex-wifid"
    cp "$REPO_ROOT/ext/TermSP/TermSP/libs/libsdlfox.so" "$app/lib/"
    cp "$REPO_ROOT/ext/TermSP/TermSP/libs/libvterm.so" "$app/lib/"
    # Sherpa STT runtime libs (aarch64, from sherpa-onnx-go-linux module)
    SHERPA_LIB_DIR="$(cd "$REPO_ROOT/src" && go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-linux 2>/dev/null)/lib/aarch64-unknown-linux-gnu"
    if [ -d "$SHERPA_LIB_DIR" ]; then
        cp "$SHERPA_LIB_DIR/libsherpa-onnx-c-api.so" "$app/lib/"
        cp "$SHERPA_LIB_DIR/libonnxruntime.so" "$app/lib/"
    else
        echo "warning: sherpa libs not found at $SHERPA_LIB_DIR — STT won't work" >&2
    fi

    # STT model files.  Parakeet is the default (better quality: punctuation +
    # casing).  Conformer-Small is optional — included when present.
    mkdir -p "$app/stt" "$app/stt-parakeet"
    PARAKEET_MODEL="$REPO_ROOT/bin/sherpa-onnx-nemo-parakeet_tdt_ctc_110m-en-36000-int8"
    if [ -f "$PARAKEET_MODEL/model.int8.onnx" ] && [ -f "$PARAKEET_MODEL/tokens.txt" ]; then
        cp "$PARAKEET_MODEL/model.int8.onnx" "$PARAKEET_MODEL/tokens.txt" "$app/stt-parakeet/"
    else
        echo "warning: Parakeet STT model not found at $PARAKEET_MODEL — " >&2
        echo "  voice dictation will show 'model not found'. Run: make install-stt-parakeet" >&2
    fi
    SHERPA_MODEL="$REPO_ROOT/bin/sherpa-onnx-nemo-ctc-en-conformer-small-2024-08-01"
    if [ -f "$SHERPA_MODEL/model.int8.onnx" ] && [ -f "$SHERPA_MODEL/tokens.txt" ]; then
        cp "$SHERPA_MODEL/model.int8.onnx" "$SHERPA_MODEL/tokens.txt" "$app/stt/"
    fi
    cp "$REPO_ROOT/ext/fonts/"*.ttf "$app/fonts/"
    cp "$REPO_ROOT/ext/brick/start-rex.sh" \
       "$REPO_ROOT/ext/brick/stop-rex.sh" \
       "$REPO_ROOT/ext/brick/debian-rex.sh" \
       "$REPO_ROOT/ext/brick/debian-sshd.sh" \
       "$REPO_ROOT/ext/brick/debian-enter.sh" \
       "$REPO_ROOT/ext/brick/debian-cleanup.sh" \
       "$REPO_ROOT/ext/brick/shutdown.sh" \
       "$REPO_ROOT/ext/brick/shutdown-host.sh" \
       "$REPO_ROOT/ext/brick/emulationstation.sh" \
       "$app/"
    cp "$REPO_ROOT/config.yaml.brick.example" "$app/config.yaml.brick.example"

    cp "$REPO_ROOT/ext/brick/Rex.sh" "$tools/Rex.sh"

    cp "$REPO_ROOT/bin/rex-brick" "$home/bin/rex"
    cp "$REPO_ROOT/config/dot/tmux.conf" "$home/.tmux.conf"
    cp "$REPO_ROOT/config/dot/tmux-wifi.sh" "$home/.tmux-wifi.sh"

    # zot-tts extension + TTS voices.  Optional — staged when built/fetched.
    # zot itself is not packaged; the extension activates once zot is installed.
    ZOT_TTS_DIR="$REPO_ROOT/bin/zot-tts"
    if [ -f "$ZOT_TTS_DIR/zot-tts" ]; then
        ext="$home/.local/state/zot/extensions/zot-tts"
        mkdir -p "$ext" "$home/models"
        cp "$ZOT_TTS_DIR/zot-tts" "$ZOT_TTS_DIR"/*.so "$ZOT_TTS_DIR/extension.json" "$ext/"
        cp "$REPO_ROOT/ext/zot-tts/AGENTS-snippet.md" "$ext/"
        chmod +x "$ext/zot-tts"
        for m in vits-piper-en_US-lessac-medium vits-piper-en_US-ryan-medium kitten-nano-en-v0_1-fp16; do
            if [ -d "$REPO_ROOT/bin/tts-models/$m" ]; then
                cp -R "$REPO_ROOT/bin/tts-models/$m" "$home/models/$m"
            else
                echo "warning: TTS voice $m not found — run: make fetch-tts-models" >&2
            fi
        done
    else
        echo "warning: bin/zot-tts not built — zot TTS won't be available. Run: make build-zot-tts fetch-tts-models" >&2
    fi

    chmod +x "$app/termsp" "$app/rex-wifid" \
        "$app/start-rex.sh" "$app/stop-rex.sh" \
        "$app/debian-rex.sh" "$app/debian-sshd.sh" \
        "$app/debian-enter.sh" "$app/debian-cleanup.sh" \
        "$app/shutdown.sh" "$app/shutdown-host.sh" "$app/emulationstation.sh" \
        "$tools/Rex.sh" "$home/bin/rex" "$home/.tmux-wifi.sh"

    cp "$INSTALL_DIR/install.sh" "$stage/install.sh"
    cp "$INSTALL_DIR/install-brick.sh" "$stage/install-brick.sh"
    chmod +x "$stage/install.sh" "$stage/install-brick.sh"

    echo "$VERSION" > "$stage/VERSION"
    echo "brick" > "$stage/TARGET"
}

stage_r36s() {
    local stage="$1"
    local p="$stage/payload"

    echo "==> staging r36s payload"
    mkdir -p "$p/bin" "$p/config" "$p/dot" "$p/systemd" "$p/opt"

    require_file "$REPO_ROOT/bin/rex" "make build-docker-r36s"

    cp "$REPO_ROOT/bin/rex" "$p/bin/rex"
    cp "$REPO_ROOT/config.yaml" "$p/config/config.yaml"
    cp "$REPO_ROOT/config/dot/"* "$p/dot/"
    cp -r "$REPO_ROOT/config/systemd/." "$p/systemd/"
    cp -r "$REPO_ROOT/config/system/opt/." "$p/opt/"

    chmod +x "$p/bin/rex"

    cp "$INSTALL_DIR/install.sh" "$stage/install.sh"
    cp "$INSTALL_DIR/install-r36s.sh" "$stage/install-r36s.sh"
    chmod +x "$stage/install.sh" "$stage/install-r36s.sh"

    echo "$VERSION" > "$stage/VERSION"
    echo "r36s" > "$stage/TARGET"
}

build_tarball() {
    local target="$1"
    local name="rex-${target}-${VERSION}"
    local stage_parent
    stage_parent="$(mktemp -d)"
    local stage="$stage_parent/$name"

    mkdir -p "$stage"

    case "$target" in
        brick) stage_brick "$stage" ;;
        r36s)  stage_r36s "$stage" ;;
        *) echo "unknown target: $target" >&2; exit 1 ;;
    esac

    local out="$DIST_DIR/$name.tar.gz"
    rm -f "$out"
    tar czf "$out" -C "$stage_parent" "$name"
    rm -rf "$stage_parent"

    local size
    size=$(du -h "$out" | cut -f1)
    echo "==> wrote $out ($size)"
}

for t in "${TARGETS[@]}"; do
    build_tarball "$t"
done
