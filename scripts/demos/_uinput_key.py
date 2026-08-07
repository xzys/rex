#!/usr/bin/env python3
"""Drive EmulationStation through a virtual uinput keyboard.

ES runs on the bare framebuffer with rex — and its control socket — shut down,
so the gamepad injection the other demos use is unavailable. The Brick's own
controller registers a kbd handler, which means ES is already listening for
key events, and a virtual keyboard needs no controller mapping the way a
virtual joystick would.

Runs on the Knulli host (needs /dev/uinput), not inside the chroot.

usage: _uinput_key.py right right 1.5 enter down down esc
       (bare numbers are pauses in seconds)
"""
import fcntl, os, struct, sys, time

UINPUT = "/dev/uinput"
EV_SYN, EV_KEY = 0x00, 0x01
SYN_REPORT = 0

KEYS = {"up": 103, "down": 108, "left": 105, "right": 106,
        "enter": 28, "a": 28, "esc": 1, "b": 1, "space": 57}

# _IOW('U', 100, int) and friends; 'U' is 0x55.
UI_SET_EVBIT = 0x40045564
UI_SET_KEYBIT = 0x40045565
UI_DEV_CREATE = 0x5501
UI_DEV_DESTROY = 0x5502

# Gap after each press. Slow enough that ES's carousel animation finishes, so
# the capture shows the scroll rather than a jump cut.
PRESS_GAP = 0.75


def emit(fd, etype, code, value):
    # struct input_event on aarch64: two longs (sec, usec), u16, u16, s32
    os.write(fd, struct.pack("qqHHi", 0, 0, etype, code, value))


def main(tokens):
    fd = os.open(UINPUT, os.O_WRONLY | os.O_NONBLOCK)
    fcntl.ioctl(fd, UI_SET_EVBIT, EV_KEY)
    for code in set(KEYS.values()):
        fcntl.ioctl(fd, UI_SET_KEYBIT, code)

    # struct uinput_user_dev: name[80], input_id{4 x u16}, u32, 4 x s32[64]
    dev = struct.pack("80sHHHHi", b"rex-demo-keyboard", 0x03, 0x1234, 0x5678, 1, 0)
    dev += b"\x00" * (64 * 4 * 4)
    os.write(fd, dev)
    fcntl.ioctl(fd, UI_DEV_CREATE)
    time.sleep(1.0)  # let ES notice the new device before typing at it

    for token in tokens:
        try:
            time.sleep(float(token))
            continue
        except ValueError:
            pass
        code = KEYS[token]
        for value in (1, 0):
            emit(fd, EV_KEY, code, value)
            emit(fd, EV_SYN, SYN_REPORT, 0)
            time.sleep(0.04)
        time.sleep(PRESS_GAP)

    time.sleep(0.4)
    fcntl.ioctl(fd, UI_DEV_DESTROY)
    os.close(fd)


if __name__ == "__main__":
    main(sys.argv[1:])
