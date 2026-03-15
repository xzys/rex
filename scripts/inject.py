#!/usr/bin/env python3
"""
Inject input events into rex via the control socket.

Usage:
    inject.py [--socket PATH] EVENT [EVENT ...]

Events:
    down:KEY    - key down
    up:KEY      - key up
    tap:KEY     - key down + up with default delay
    sleep:MS    - sleep for MS milliseconds

Key names (case-insensitive):
    DPadUp, DPadDown, DPadLeft, DPadRight
    A, B, X, Y
    L1, R1, L2, R2
    Select, Start, Fn

Examples:
    inject.py down:Select sleep:200 up:Select
    inject.py tap:A tap:B
    inject.py --socket /tmp/rex-control.sock tap:Select

Can also read events from stdin (one per line):
    echo "tap:Select" | inject.py
    cat replay.txt | inject.py
"""

import argparse
import json
import socket
import sys
import time

KEY_CODES = {
    "dpadup":    0,
    "dpaddown":  1,
    "dpadleft":  2,
    "dpadright": 3,
    "a":         4,
    "b":         5,
    "x":         6,
    "y":         7,
    "l1":        8,
    "r1":        9,
    "l2":        10,
    "r2":        11,
    "select":    12,
    "start":     13,
    "fn":        14,
}

DEFAULT_SOCKET = "/tmp/rex-control.sock"
DEFAULT_TAP_DELAY_MS = 50


def parse_events(tokens):
    events = []
    for token in tokens:
        token = token.strip()
        if not token or token.startswith("#"):
            continue
        if ":" not in token:
            print(f"error: invalid event '{token}' (expected format action:value)", file=sys.stderr)
            sys.exit(1)
        action, _, value = token.partition(":")
        action = action.lower()
        if action == "sleep":
            events.append(("sleep", int(value)))
        elif action in ("down", "up", "tap"):
            key_name = value.lower()
            if key_name not in KEY_CODES:
                print(f"error: unknown key '{value}'", file=sys.stderr)
                print(f"valid keys: {', '.join(KEY_CODES.keys())}", file=sys.stderr)
                sys.exit(1)
            events.append((action, KEY_CODES[key_name]))
        else:
            print(f"error: unknown action '{action}'", file=sys.stderr)
            sys.exit(1)
    return events


def send_message(sock, msg):
    sock.sendall(json.dumps(msg).encode() + b"\n")


def run(socket_path, events):
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.connect(socket_path)
    try:
        for action, value in events:
            if action == "sleep":
                time.sleep(value / 1000.0)
            elif action == "down":
                send_message(s, {"type": "inject_input", "key": value, "is_down": True})
            elif action == "up":
                send_message(s, {"type": "inject_input", "key": value, "is_down": False})
            elif action == "tap":
                send_message(s, {"type": "inject_input", "key": value, "is_down": True})
                time.sleep(DEFAULT_TAP_DELAY_MS / 1000.0)
                send_message(s, {"type": "inject_input", "key": value, "is_down": False})
    finally:
        s.close()


def main():
    parser = argparse.ArgumentParser(description="Inject input events into rex control socket")
    parser.add_argument("--socket", default=DEFAULT_SOCKET, help="Control socket path")
    parser.add_argument("events", nargs="*", help="Events to inject")
    args = parser.parse_args()

    if args.events:
        tokens = args.events
    elif not sys.stdin.isatty():
        tokens = sys.stdin.read().split()
    else:
        parser.print_help()
        sys.exit(1)

    events = parse_events(tokens)
    run(args.socket, events)


if __name__ == "__main__":
    main()
