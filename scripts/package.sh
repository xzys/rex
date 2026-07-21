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
    require_file "$REPO_ROOT/bin/termsp" "make build-docker-termsp"
    require_file "$REPO_ROOT/ext/TermSP/TermSP/libs/libsdlfox.so" \
        "git submodule update --init ext/TermSP/TermSP"
    require_file "$REPO_ROOT/ext/TermSP/TermSP/libs/libvterm.so" \
        "git submodule update --init ext/TermSP/TermSP"

    cp "$REPO_ROOT/bin/termsp" "$app/termsp"
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
    cp "$REPO_ROOT/ext/fonts/"*.ttf "$app/fonts/"
    cp "$REPO_ROOT/ext/brick/start-rex.sh" \
       "$REPO_ROOT/ext/brick/stop-rex.sh" \
       "$REPO_ROOT/ext/brick/debian-rex.sh" \
       "$REPO_ROOT/ext/brick/debian-sshd.sh" \
       "$REPO_ROOT/ext/brick/debian-enter.sh" \
       "$REPO_ROOT/ext/brick/debian-cleanup.sh" \
       "$REPO_ROOT/ext/brick/shutdown.sh" \
       "$REPO_ROOT/ext/brick/shutdown-host.sh" \
       "$REPO_ROOT/ext/brick/games.sh" \
       "$REPO_ROOT/ext/brick/connman-rex.conf" \
       "$app/"
    cp "$REPO_ROOT/config.yaml.brick.example" "$app/config.yaml.brick.example"

    cp "$REPO_ROOT/ext/brick/Rex.sh" "$tools/Rex.sh"

    cp "$REPO_ROOT/bin/rex-brick" "$home/bin/rex"
    cp "$REPO_ROOT/config/dot/tmux.conf" "$home/.tmux.conf"
    cp "$REPO_ROOT/config/dot/tmux-wifi.sh" "$home/.tmux-wifi.sh"

    chmod +x "$app/termsp" \
        "$app/start-rex.sh" "$app/stop-rex.sh" \
        "$app/debian-rex.sh" "$app/debian-sshd.sh" \
        "$app/debian-enter.sh" "$app/debian-cleanup.sh" \
        "$app/shutdown.sh" "$app/shutdown-host.sh" "$app/games.sh" \
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
