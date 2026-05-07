#!/bin/sh
# Print wifi status for the tmux status bar: a unicode sparkline of recent
# signal samples + current dBm. Reads via `iw` (nl80211) since the Knulli
# kernel exposes neither /proc/net/wireless nor /sys/class/net/*/wireless/*.
export LC_ALL=C.UTF-8
iface=wlan0
hist=${TMPDIR:-/tmp}/.tmux-wifi-hist
maxlen=10

carrier=$(cat /sys/class/net/$iface/carrier 2>/dev/null)
if [ "$carrier" != "1" ]; then
  rm -f "$hist"
  printf 'no wifi'
  exit 0
fi

dbm=$(/usr/sbin/iw dev $iface link 2>/dev/null | awk '/signal:/ {print $2; exit}')
case "$dbm" in
  -[0-9]*) ;;
  *) dbm=-100 ;;
esac

{ [ -f "$hist" ] && cat "$hist"; echo "$dbm"; } | tail -n $maxlen > "$hist.tmp" && mv "$hist.tmp" "$hist"

# Map dBm in [-90, -30] → bar 0..7. ▁=weakest, █=strongest.
bars="▁ ▂ ▃ ▄ ▅ ▆ ▇ █"
spark=""
while read -r v; do
  [ "$v" -gt -30 ] 2>/dev/null && v=-30
  [ "$v" -lt -90 ] 2>/dev/null && v=-90
  idx=$(( (v + 90) * 7 / 60 ))
  [ "$idx" -lt 0 ] && idx=0
  [ "$idx" -gt 7 ] && idx=7
  bar=$(printf '%s\n' "$bars" | cut -d' ' -f$((idx + 1)))
  spark="${spark}${bar}"
done < "$hist"

printf '%s %s' "$spark" "$dbm"
