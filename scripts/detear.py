#!/usr/bin/env python3
"""Drop torn frames from a framebuffer capture.

Tearing can't be avoided at the source on the Brick. The display is double
buffered (yoffset alternates 0/768) but EmulationStation writes to whichever
page it likes: reading the same page twice in a row returns different bytes
about a third of the time, for both the displayed and the undisplayed page.
FBIO_WAITFORVSYNC exists but is a stub — 60 calls return in a millisecond — so
there is nothing to sync a read against.

What is left is to notice the bad frames and throw them away. A torn frame is
stitched from two source frames: rows above the tear match the previous frame,
rows below match the next one, which shows up as a step in the per-row
difference profile. Dropping ~10% of frames and holding the previous one reads
far better than a visible tear.

usage: detear.py in.mp4 out.mp4 [fps]
"""
import os, subprocess, sys

W, H = 128, 96
THRESHOLD = float(os.environ.get("DETEAR_THRESHOLD", 4.0))  # step in the row-difference profile
MOTION_FLOOR = 2.0  # below this the frame is static and cannot tear


def gray_frames(path):
    p = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", path,
         "-vf", f"scale={W}:{H},format=gray", "-f", "rawvideo", "-"],
        capture_output=True, check=True)
    buf = p.stdout
    return [buf[i * W * H:(i + 1) * W * H] for i in range(len(buf) // (W * H))]


def row_diff(a, b):
    return [sum(abs(a[r * W + c] - b[r * W + c]) for c in range(W)) / W
            for r in range(H)]


def torn_frames(frames):
    bad = []
    for i in range(1, len(frames) - 1):
        d_prev = row_diff(frames[i], frames[i - 1])
        d_next = row_diff(frames[i], frames[i + 1])
        if sum(d_prev) / H < MOTION_FLOOR:
            continue
        best = 0.0
        for split in range(8, H - 8):
            tp = sum(d_prev[:split]) / split
            tn = sum(d_next[:split]) / split
            bp = sum(d_prev[split:]) / (H - split)
            bn = sum(d_next[split:]) / (H - split)
            best = max(best, (tn - tp) + (bp - bn), (tp - tn) + (bn - bp))
        if best > THRESHOLD:
            bad.append(i)
    return bad


def main(src, dst, fps):
    frames = gray_frames(src)
    bad = set(torn_frames(frames))
    print(f"{len(frames)} frames, {len(bad)} torn ({100 * len(bad) / max(1, len(frames)):.1f}%)")
    if not bad:
        subprocess.run(["ffmpeg", "-v", "error", "-i", src, "-c", "copy", "-y", dst], check=True)
        return

    # Drop the torn frames but leave the surviving timestamps alone, so `fps`
    # refills each gap by holding the previous frame. Renumbering instead
    # (setpts=N/FRAME_RATE/TB) closes the gaps and speeds the whole clip up —
    # dropping 30% of frames made a 24s capture play back in 16s.
    keep = "+".join(f"eq(n\\,{i})" for i in sorted(bad))
    subprocess.run([
        "ffmpeg", "-v", "error", "-i", src,
        "-vf", f"select='not({keep})',fps={fps}",
        "-c:v", "libx264", "-preset", "slow", "-crf", "24",
        "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-y", dst,
    ], check=True)
    print(f"wrote {dst}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2], sys.argv[3] if len(sys.argv) > 3 else "30")
