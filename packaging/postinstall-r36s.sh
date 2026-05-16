#!/bin/sh
# Runs after `apt install rex-r36s` on dArkOSRE.
# Reloads systemd so the freshly-installed rex.service is visible.
# Does NOT enable the service — user opts in.

set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
fi

cat <<'EOF'
rex installed at /usr/bin/rex.

Service is installed but not enabled. To start:
  sudo systemctl enable --now rex.service

Templates available at /usr/share/rex/:
  tmux.conf, tmux-wifi.sh, config.yaml.example

Copy templates into your home if you don't already have them:
  mkdir -p ~/.config/rex
  cp /usr/share/rex/config.yaml.example ~/.config/rex/config.yaml
  cp /usr/share/rex/tmux.conf ~/.tmux.conf
  cp /usr/share/rex/tmux-wifi.sh ~/.tmux-wifi.sh
EOF

exit 0
