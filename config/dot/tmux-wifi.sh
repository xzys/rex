#!/bin/sh
# Print wifi status for the tmux status bar: 'off' when no link, otherwise the
# SSID. Reads via `iw` (nl80211) since the Knulli kernel exposes neither
# /proc/net/wireless nor /sys/class/net/*/wireless/*.
export LC_ALL=C.UTF-8
iface=wlan0

carrier=$(cat /sys/class/net/$iface/carrier 2>/dev/null)
if [ "$carrier" != "1" ]; then
  printf 'off'
  exit 0
fi

ssid=$(/usr/sbin/iw dev $iface link 2>/dev/null | awk -F': ' '/^\tSSID:/ {print $2; exit}')
if [ -z "$ssid" ]; then
  printf 'off'
else
  printf '%s' "$ssid"
fi
