#!/bin/sh
# Print wifi status for the tmux status bar. Queries the rex-wifid daemon
# over its Unix socket (visible inside the chroot via /tmp bind mount).
# Falls back to 'off' if the daemon isn't reachable.
export LC_ALL=C.UTF-8

out=$(python3 -c "
import socket, json
try:
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.settimeout(1)
    s.connect('/tmp/rex-wifi.sock')
    s.sendall(b'{\"id\":\"1\",\"cmd\":\"status\"}\n')
    resp = json.loads(s.recv(4096))
    print(resp.get('ssid',''))
except:
    print('')
" 2>/dev/null)

if [ -z "$out" ]; then
    printf 'off'
else
    printf '%s' "$out"
fi
