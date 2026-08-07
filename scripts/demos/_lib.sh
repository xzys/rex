# Shared helpers for demo drive scripts, sourced on the device as
# /tmp/rex-demo-lib.sh (record-brick.sh puts it there).
#
# Button presses go to rex's control socket, which hands them to the same
# operator the physical buttons feed, so demos exercise the real input path —
# menu navigation, per-app key plugins, the global fallback — rather than
# typing commands into a shell.

cat > /tmp/rex-demo-pad.py <<'PY'
import json, socket, sys, time

KEYS = {"up": 0, "down": 1, "left": 2, "right": 3, "a": 4, "b": 5, "x": 6,
        "y": 7, "l1": 8, "r1": 9, "l2": 10, "r2": 11, "select": 12,
        "start": 13, "fn": 14}

s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect("/tmp/rex-control.sock")
for name in sys.argv[1:]:
    for down in (True, False):
        msg = {"type": "inject_input", "key": KEYS[name], "is_down": down}
        s.sendall((json.dumps(msg) + "\n").encode())
        time.sleep(0.05)
s.close()
PY

# pad BUTTON [SETTLE] — press a gamepad button, then wait for the UI.
pad() { python3 /tmp/rex-demo-pad.py "$1"; sleep "${2:-0.8}"; }

# type_text STRING — type a character at a time so the recording shows typing
# rather than a paste appearing all at once.
type_text() {
    s=$1
    while [ -n "$s" ]; do
        c=${s%"${s#?}"}
        s=${s#?}
        if [ "$c" = " " ]; then
            tmux send-keys -t rex Space
        else
            tmux send-keys -t rex -l -- "$c"
        fi
        sleep 0.03
    done
}

# reset_windows — back to a single shell window, so every take starts the same.
reset_windows() {
    for w in 5 4 3 2 1; do
        tmux kill-window -t "rex:$w" 2>/dev/null
    done
}
