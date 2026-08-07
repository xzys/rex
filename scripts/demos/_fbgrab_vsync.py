#!/usr/bin/env python3
"""Stream the visible framebuffer page to stdout, synchronised to page flips.

ffmpeg's fbdev input reads the buffer whenever it likes. The Brick's display is
double buffered — yoffset alternates between 0 and 768 — so a read can straddle
a flip and come back stitched from two frames, which is the tearing visible
during EmulationStation's carousel scroll.

FBIO_WAITFORVSYNC is accepted by this driver but is a stub: 60 calls return in
about a millisecond, so it syncs to nothing. Instead this polls the pan offset
and copies only just after it changes, and re-reads the page until two samples
of it agree, on the theory that a page holding still is a page nothing is
drawing into.

DOES NOT FIX TEARING. Measured over 360 frames of carousel scroll: plain ffmpeg
fbdev 40 torn, this 39-46. The premise is wrong — ES writes to both pages
regardless of which is on screen, so there is no safe moment to read. Reading
the same page twice in a row returns different bytes in about a third of
attempts, for the displayed page (12/40) and the undisplayed one (14/40) alike.

It is kept because it does capture a complete frame set where ffmpeg's own
fbdev input drops frames (360/360 against 300/360 over 12s), and because the
next person to try fixing the tearing should not have to rediscover the above.

Writes raw bgra frames. Pipe into ffmpeg:

  _fbgrab_vsync.py 30 720 | ffmpeg -f rawvideo -pix_fmt bgra \\
      -s 1024x768 -framerate 30 -i - ...

usage: _fbgrab_vsync.py [FPS] [FRAMES]
"""
import fcntl, mmap, os, struct, sys, time

FB = "/dev/fb0"
FBIOGET_VSCREENINFO = 0x4600
VSCREENINFO_LEN = 160  # sizeof(struct fb_var_screeninfo)

# Give up waiting for a flip after this long and copy anyway: a still screen
# never flips, and a still screen cannot tear.
STATIC_TIMEOUT = 0.1

# How many times to re-read a page hoping it holds still before giving up.
STABLE_TRIES = 3


def main(fps, frames):
    fd = os.open(FB, os.O_RDONLY)
    info = bytearray(VSCREENINFO_LEN)

    def yoffset():
        fcntl.ioctl(fd, FBIOGET_VSCREENINFO, info, True)
        return struct.unpack_from("6I", info, 0)[5]

    fcntl.ioctl(fd, FBIOGET_VSCREENINFO, info, True)
    yres = struct.unpack_from("6I", info, 0)[1]
    stride = int(open("/sys/class/graphics/fb0/stride").read().strip())
    page = yres * stride

    # st_size reads as 0 for this device; the pan window reaches well past one
    # screen, so map the whole 64MB region the driver reports.
    mm = mmap.mmap(fd, 64 * 1024 * 1024, mmap.MAP_SHARED, mmap.PROT_READ)
    view = memoryview(mm)

    # The panel is ~60Hz and flips once per refresh; take every Nth flip.
    every = max(1, round(60.0 / fps))

    # Sampling rows rather than the whole page keeps the stability check cheap:
    # a tear is a band many rows tall, so a spread of rows always straddles it.
    probe_rows = range(0, yres, 8)

    def sample(start):
        return b"".join(bytes(view[start + r * stride:start + r * stride + 256])
                        for r in probe_rows)

    out = sys.stdout.buffer
    last = yoffset()
    prev_good = None
    try:
        for _ in range(frames):
            for _ in range(every):
                deadline = time.time() + STATIC_TIMEOUT
                while True:
                    cur = yoffset()
                    if cur != last:
                        last = cur
                        break
                    if time.time() > deadline:
                        cur = last
                        break
                    time.sleep(0.001)
            start = cur * stride

            # Both pages get written regardless of which is on screen, so the
            # only reliable test is whether the page held still while we looked
            # at it. Two matching samples means nothing was drawing into it.
            frame = None
            for _ in range(STABLE_TRIES):
                a = sample(start)
                if a == sample(start):
                    frame = view[start:start + page]
                    break
            if frame is None:
                # Still being drawn into; repeat the last clean frame rather
                # than emit a torn one.
                frame = prev_good
                if frame is None:
                    continue
            else:
                prev_good = bytes(frame)
            out.write(frame)
    except (BrokenPipeError, KeyboardInterrupt):
        pass
    finally:
        out.flush()
        view.release()
        mm.close()
        os.close(fd)


if __name__ == "__main__":
    main(float(sys.argv[1]) if len(sys.argv) > 1 else 30,
         int(sys.argv[2]) if len(sys.argv) > 2 else 300)
