#!/usr/bin/env bash
# Package a built kernel Image into a flashable Brick boot.img.
#
#   ./scripts/brick/make-image.sh <Image> <NN-name> "<one-line description>"
#
# Reuses Knulli's ramdisk and header parameters, so the result boots the same rootfs the
# device already has -- only the kernel changes. The SHA1 at header offset 576 is
# recomputed by bootimg.py; U-Boot validates it and silently refuses the image otherwise.
#
# See notes/brick-flashing-runbook.md
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
KERNEL="${1:?usage: make-image.sh <Image> <NN-name> <description>}"
NAME="${2:?usage: make-image.sh <Image> <NN-name> <description>}"
DESC="${3:-}"

FW="$REPO/targets/trimui-brick/firmware/bootimg"
OUT="$REPO/targets/trimui-brick/images/$NAME"
[ -f "$KERNEL" ] || { echo "no kernel at $KERNEL" >&2; exit 1; }
[ -f "$FW/boot.img-ramdisk.gz" ] || { echo "missing stock parts in $FW" >&2; exit 1; }
mkdir -p "$OUT/parts"

cp "$KERNEL" "$OUT/parts/kernel"
cp "$FW/boot.img-ramdisk.gz" "$OUT/parts/ramdisk"

# Rebuild the 2048-byte header from Knulli's recorded parameters.
python3 - "$FW" "$OUT/parts" <<'PY'
import struct, sys, os
fw, parts = sys.argv[1], sys.argv[2]
val = lambda n: open(os.path.join(fw, n)).read().strip()
M = 0xffffffff
page = int(val('boot.img-pagesize')); base = int(val('boot.img-base'), 16)
kern = open(os.path.join(parts, 'kernel'), 'rb').read()
rdk  = open(os.path.join(parts, 'ramdisk'), 'rb').read()
hdr = bytearray(page); hdr[0:8] = b'ANDROID!'
struct.pack_into('<8I', hdr, 8,
    len(kern), (base + int(val('boot.img-kerneloff'), 16)) & M,
    len(rdk),  (base + int(val('boot.img-ramdiskoff'), 16)) & M,
    0,         (base + int(val('boot.img-secondoff'), 16)) & M,
    (base + int(val('boot.img-tagsoff'), 16)) & M, page)
b = val('boot.img-board').encode(); c = val('boot.img-cmdline').encode()
hdr[48:48+len(b)] = b; hdr[64:64+len(c)] = c
open(os.path.join(parts, 'header.bin'), 'wb').write(bytes(hdr))
PY

"$REPO/scripts/brick/bootimg.py" repack "$OUT/parts" "$OUT/boot.img"
( cd "$OUT" && sha256sum boot.img > boot.img.sha256 )
rm -rf "$OUT/parts"

echo
"$REPO/scripts/brick/bootimg.py" info "$OUT/boot.img"
echo
echo "image: $OUT/boot.img   ($DESC)"
echo "flash: ./scripts/brick/flash-image.sh $OUT/boot.img /dev/sdX"
