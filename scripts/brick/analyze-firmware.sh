#!/usr/bin/env bash
# One command from "firmware just arrived" to "Ghidra has looked for the panel driver".
#
#   ./scripts/brick/analyze-firmware.sh targets/trimui-brick/firmware/boot.img
#   ./scripts/brick/analyze-firmware.sh targets/trimui-brick/firmware/boot_package.fex
#
# Handles either input:
#   boot.img          -> unpack kernel+ramdisk, analyse the kernel
#   boot_package.fex  -> carve, then analyse the largest payload as U-Boot
#
# Everything lands under targets/trimui-brick/extracted/<name>/ (gitignored).
# Background: notes/ghidra-linux-port-plan.md section 5.4
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IN="${1:?usage: analyze-firmware.sh <boot.img|boot_package.fex|kernel>}"
[ -f "$IN" ] || { echo "no such file: $IN" >&2; exit 1; }

NAME="$(basename "$IN" | tr -c 'A-Za-z0-9._-' '_')"
OUT="$REPO/targets/trimui-brick/extracted/$NAME"
mkdir -p "$OUT"

echo "== provenance =="
sha256sum "$IN" | tee "$OUT/source.sha256"
ls -la "$IN"

MAGIC=$(head -c 8 "$IN")
KERNEL=""

if [ "$MAGIC" = "ANDROID!" ]; then
    echo "== Android bootimg =="
    "$REPO/scripts/brick/bootimg.py" info   "$IN"
    "$REPO/scripts/brick/bootimg.py" unpack "$IN" "$OUT/bootimg"
    KERNEL="$OUT/bootimg/kernel"
else
    echo "== not a bootimg; treating as a container =="
    if command -v binwalk >/dev/null 2>&1; then
        binwalk -e --directory="$OUT" "$IN" || true
    else
        echo "binwalk not installed -- falling back to a magic scan" >&2
        # ANDROID! / gzip / ARM64 kernel 'ARMd' magic at +0x38
        grep -abo -e 'ANDROID!' -e $'\x1f\x8b\x08' "$IN" 2>/dev/null | head -20 \
            | tee "$OUT/magic-offsets.txt" || true
        echo "inspect $OUT/magic-offsets.txt and carve with dd" >&2
    fi
    KERNEL="$(find "$OUT" -type f -size +256k -printf '%s %p\n' 2>/dev/null \
              | sort -rn | head -1 | cut -d' ' -f2- || true)"
fi

if [ -z "$KERNEL" ] || [ ! -f "$KERNEL" ]; then
    echo "no payload identified to analyse; stopping after extraction." >&2
    exit 0
fi

echo "== payload =="
ls -la "$KERNEL"
# arm64 Image carries "ARMd" (0x644d5241) at offset 0x38
if [ "$(dd if="$KERNEL" bs=1 skip=56 count=4 2>/dev/null | xxd -p)" = "41524d64" ]; then
    echo "-> arm64 Linux Image magic found at 0x38"
fi

echo "== Ghidra: hunting for the panel driver =="
# shellcheck source=/dev/null
. "$REPO/scripts/ghidra/env.sh"
PROJ="$OUT/ghidra"; mkdir -p "$PROJ"

"$GHIDRA_HEADLESS" "$PROJ" panel \
    -import "$KERNEL" \
    -processor AARCH64:LE:64:v8A \
    -loader BinaryLoader \
    -loader-baseAddr "${BASE_ADDR:-0x40080000}" \
    -scriptPath "$REPO/scripts/ghidra" \
    -postScript FindPanelInit.java "${PANEL:-jd9168}" \
    -log "$OUT/ghidra.log" \
    2>&1 | tee "$OUT/panel-search.txt" | grep -E 'PANEL_SEARCH|STRING |RAWHIT|XREF|candidate' || true

echo
echo "== done =="
echo "  extraction : $OUT"
echo "  ghidra log : $OUT/ghidra.log"
echo "  hits       : $OUT/panel-search.txt"
echo
echo "If FindPanelInit found nothing, the sequence is still there -- it is a long run of"
echo "identical calls rather than a string. See notes/brick-display-repoint-design.md,"
echo "'Consequences for the Ghidra hunt'."
