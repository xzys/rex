#!/usr/bin/env python3
"""Unpack (and repack) an Android boot image -- the format the TrimUI Brick uses.

Deliberately dependency-free: `abootimg` is not installed on the analysis host and
installing it needs sudo. Everything here is stdlib.

The Brick's boot.img is a standard Android bootimg v0:
    magic "ANDROID!", page_size 2048, board "sun50i_arm64",
    kernel @ 0x40080000, ramdisk @ 0x42000000

The trap that cost a day in Apr 2026: the header carries a **SHA1 at offset 576** over
kernel+len+ramdisk+len+second+len, and U-Boot validates it. Omit or miscompute it and the
device hangs on the boot logo. `repack` here recomputes it; `verify` checks it.

    ./bootimg.py info    boot.img
    ./bootimg.py unpack  boot.img outdir/
    ./bootimg.py repack  outdir/ new-boot.img [--kernel Image]
"""
import hashlib
import os
import struct
import sys

MAGIC = b"ANDROID!"


def _pad(n, page):
    return (page - (n % page)) % page


def parse(data):
    if data[:8] != MAGIC:
        raise SystemExit("not an Android boot image (magic %r)" % data[:8])
    f = struct.unpack_from("<8I", data, 8)
    h = {
        "kernel_size": f[0], "kernel_addr": f[1],
        "ramdisk_size": f[2], "ramdisk_addr": f[3],
        "second_size": f[4], "second_addr": f[5],
        "tags_addr": f[6], "page_size": f[7],
    }
    h["header_version"], h["os_version"] = struct.unpack_from("<2I", data, 40)
    h["name"] = data[48:64].rstrip(b"\0").decode("ascii", "replace")
    h["cmdline"] = data[64:576].rstrip(b"\0").decode("ascii", "replace")
    h["sha1"] = data[576:596].hex()
    h["extra_cmdline"] = data[608:1632].rstrip(b"\0").decode("ascii", "replace")
    return h


def _sections(data, h):
    p = h["page_size"]
    off = p
    out = {}
    for name in ("kernel", "ramdisk", "second"):
        size = h[name + "_size"]
        out[name] = data[off:off + size] if size else b""
        off += size + _pad(size, p)
    return out


def compute_sha1(kernel, ramdisk, second):
    """Exactly what the bootloader checks: each blob followed by its little-endian length."""
    d = hashlib.sha1()
    for blob in (kernel, ramdisk, second):
        d.update(blob)
        d.update(struct.pack("<I", len(blob)))
    return d.digest()


def cmd_info(path):
    data = open(path, "rb").read()
    h = parse(data)
    s = _sections(data, h)
    for k in ("name", "page_size", "header_version", "kernel_size", "kernel_addr",
              "ramdisk_size", "ramdisk_addr", "second_size", "tags_addr"):
        v = h[k]
        print("%-16s %s" % (k, hex(v) if isinstance(v, int) and k.endswith("addr") else v))
    print("%-16s %s" % ("cmdline", h["cmdline"]))
    if h["extra_cmdline"]:
        print("%-16s %s" % ("extra_cmdline", h["extra_cmdline"]))
    want = compute_sha1(s["kernel"], s["ramdisk"], s["second"]).hex()
    ok = "OK" if want == h["sha1"] else "MISMATCH (expected %s)" % want
    print("%-16s %s  %s" % ("sha1", h["sha1"], ok))


def cmd_unpack(path, outdir):
    data = open(path, "rb").read()
    h = parse(data)
    s = _sections(data, h)
    os.makedirs(outdir, exist_ok=True)
    for name, blob in s.items():
        if blob:
            with open(os.path.join(outdir, name), "wb") as fh:
                fh.write(blob)
            print("wrote %s (%d bytes)" % (os.path.join(outdir, name), len(blob)))
    with open(os.path.join(outdir, "header.txt"), "w") as fh:
        for k, v in h.items():
            fh.write("%s=%s\n" % (k, v))
    # keep the original header verbatim so repack can preserve every field
    with open(os.path.join(outdir, "header.bin"), "wb") as fh:
        fh.write(data[:h["page_size"]])
    print("wrote %s/header.txt, %s/header.bin" % (outdir, outdir))


def cmd_repack(indir, out, kernel_override=None):
    hdr = bytearray(open(os.path.join(indir, "header.bin"), "rb").read())
    h = parse(bytes(hdr))
    page = h["page_size"]

    def rd(name, override=None):
        p = override or os.path.join(indir, name)
        return open(p, "rb").read() if os.path.exists(p) else b""

    kernel = rd("kernel", kernel_override)
    ramdisk = rd("ramdisk")
    second = rd("second")

    struct.pack_into("<I", hdr, 8, len(kernel))
    struct.pack_into("<I", hdr, 16, len(ramdisk))
    struct.pack_into("<I", hdr, 24, len(second))
    hdr[576:596] = compute_sha1(kernel, ramdisk, second)

    with open(out, "wb") as fh:
        fh.write(hdr)
        fh.write(b"\0" * _pad(len(hdr), page))
        for blob in (kernel, ramdisk, second):
            if blob:
                fh.write(blob)
                fh.write(b"\0" * _pad(len(blob), page))
    print("wrote %s (%d bytes), sha1 %s" % (out, os.path.getsize(out), hdr[576:596].hex()))


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 1
    cmd = sys.argv[1]
    if cmd == "info":
        cmd_info(sys.argv[2])
    elif cmd == "unpack":
        cmd_unpack(sys.argv[2], sys.argv[3])
    elif cmd == "repack":
        k = None
        if "--kernel" in sys.argv:
            k = sys.argv[sys.argv.index("--kernel") + 1]
        cmd_repack(sys.argv[2], sys.argv[3], k)
    else:
        print(__doc__)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
