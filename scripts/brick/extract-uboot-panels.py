#!/usr/bin/env python3
"""Extract MIPI-DSI panel initialisation tables from Allwinner sun50iw10 U-Boot.

TrimUI's U-Boot does not hardcode one panel. It probes the panel's RDID
(DCS 0xDA/0xDB/0xDC) and dispatches to one of several init tables -- the strings
`check_lcd_module`, `LCD module jd9168`, `LCD module otm1289a`, `LCD module st7703` ...
are all present in the binary. The Brick answers 91/68/03, i.e. Jadard JD9168.

Each table is an array of fixed-size records:

    offset  size  field
    0       4     DCS command       (u32 LE; 0xFE = delay pseudo-op)
    4       4     parameter count   (u32 LE; 0xFE when command is a delay)
    8       n     parameters        (zero-padded to the record size)
    --      --    record stride 0x48 (72 bytes)

Usage:
    ./extract-uboot-panels.py u-boot.bin --list
    ./extract-uboot-panels.py u-boot.bin --at 0x9e88c > jd9168-seq.txt

Feed the output to mkpanel.py to get a compilable BSP panel driver.
Background: notes/brick-display-repoint-design.md
"""
import struct
import sys

STRIDE = 0x48
DELAY_OP = 0xFE

# password / unlock first-records that identify a controller family
SIGNATURES = {
    (0xDF, b"\x91\x68\xf9"): "Jadard JD9168",
    (0xFF, b"\x12\x89\x01"): "Orise OTM1289A",
    (0xFF, b"\x98\x81\x03"): "Ilitek ILI9881",
    (0xB9, b"\xf1\x12\x87"): "Sitronix ST7703",
}


def valid(d, off):
    if off < 0 or off + STRIDE > len(d):
        return False
    cmd, ln = struct.unpack_from("<II", d, off)
    if cmd > 0xFF:
        return False
    if cmd == DELAY_OP:
        return ln == DELAY_OP
    if ln > STRIDE - 8:
        return False
    return d[off + 8 + ln:off + STRIDE] == b"\0" * (STRIDE - 8 - ln)


def bounds(d, seed):
    start = seed
    while valid(d, start - STRIDE):
        start -= STRIDE
    end = seed
    while valid(d, end):
        end += STRIDE
    return start, end


def find_tables(d, minimum=8):
    runs, off = [], 0
    while off + STRIDE <= len(d):
        if valid(d, off):
            s = off
            while valid(d, off):
                off += STRIDE
            if (off - s) // STRIDE >= minimum:
                runs.append((s, off))
        else:
            off += 4
    return runs


def describe(d, start):
    cmd, ln = struct.unpack_from("<II", d, start)
    params = d[start + 8:start + 8 + min(ln, 8)]
    for (c, p), nm in SIGNATURES.items():
        if cmd == c and params.startswith(p):
            return nm
    return "unknown"


def dump(d, start, end, out):
    for off in range(start, end, STRIDE):
        cmd, ln = struct.unpack_from("<II", d, off)
        if cmd == DELAY_OP:
            out.write("delay %d\n" % d[off + 8])
            continue
        params = d[off + 8:off + 8 + ln]
        line = "%02X" % cmd
        if params:
            line += " " + " ".join("%02X" % b for b in params)
        out.write(line + "\n")


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 1
    d = open(sys.argv[1], "rb").read()

    if "--list" in sys.argv:
        for s, e in find_tables(d):
            n = (e - s) // STRIDE
            print("0x%06x  %3d records  %s" % (s, n, describe(d, s)))
        return 0

    if "--at" in sys.argv:
        seed = int(sys.argv[sys.argv.index("--at") + 1], 0)
        s, e = bounds(d, seed)
        sys.stderr.write("table 0x%x..0x%x  %d records  %s\n"
                         % (s, e, (e - s) // STRIDE, describe(d, s)))
        dump(d, s, e, sys.stdout)
        return 0

    print(__doc__)
    return 1


if __name__ == "__main__":
    sys.exit(main())
