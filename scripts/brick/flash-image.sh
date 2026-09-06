#!/usr/bin/env bash
# Flash a boot image to a Brick SD card, defensively.
#   ./scripts/brick/flash-image.sh <boot.img> /dev/sdX
# See notes/brick-flashing-runbook.md
set -euo pipefail

IMG="${1:?usage: flash-image.sh <boot.img> <device>}"
DEV="${2:?usage: flash-image.sh <boot.img> <device>}"
SEEK=73728          # sector 73728 = 36 MB. NOT 160 -- that is the GPT header.
COUNT=32768

[ -f "$IMG" ] || { echo "no such image: $IMG" >&2; exit 1; }
[ -b "$DEV" ] || { echo "$DEV is not a block device" >&2; exit 1; }

case "$DEV" in
    *[0-9]) echo "refusing: $DEV looks like a partition; pass the whole disk" >&2; exit 1 ;;
esac

if [ "$(head -c 8 "$IMG")" != "ANDROID!" ]; then
    echo "refusing: $IMG is not an Android boot image" >&2; exit 1
fi

SIZE=$(blockdev --getsize64 "$DEV" 2>/dev/null || echo 0)
if [ "$SIZE" -gt 137438953472 ]; then
    echo "refusing: $DEV is $((SIZE/1024/1024/1024))GB -- too big to be an SD card." >&2
    echo "If this really is the card, flash manually." >&2
    exit 1
fi

echo "device : $DEV ($((SIZE/1024/1024)) MB)"
echo "image  : $IMG ($(stat -c%s "$IMG") bytes)"
echo "offset : sector $SEEK (36 MB)"
read -r -p "Type YES to write: " ok
[ "$ok" = "YES" ] || { echo "aborted"; exit 1; }

BAK="boot-backup-$(date +%Y%m%d-%H%M%S).img"
echo "backing up current boot partition to $BAK ..."
dd if="$DEV" bs=512 skip=$SEEK count=$COUNT of="$BAK" status=none
echo "backup: $(stat -c%s "$BAK") bytes"

echo "writing ..."
dd if="$IMG" of="$DEV" bs=512 seek=$SEEK conv=notrunc status=progress
sync

if [ "$(dd if="$DEV" bs=512 skip=$SEEK count=1 status=none | head -c 8)" = "ANDROID!" ]; then
    echo "verified: ANDROID! magic present at sector $SEEK"
    echo "restore with: dd if=$BAK of=$DEV bs=512 seek=$SEEK conv=notrunc"
else
    echo "WARNING: magic not found after write -- do NOT boot; restore from $BAK" >&2
    exit 1
fi
