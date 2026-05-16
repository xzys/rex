#!/bin/sh
# Runs after `apt install rex` inside the Debian chroot.
# Idempotent: just prints next steps. Does not touch /home.

set -e

cat <<'EOF'
rex installed at /usr/bin/rex.

Templates available at /usr/share/rex/:
  tmux.conf             ->  cp /usr/share/rex/tmux.conf ~/.tmux.conf
  tmux-wifi.sh          ->  cp /usr/share/rex/tmux-wifi.sh ~/.tmux-wifi.sh
  config.yaml.example   ->  mkdir -p ~/.config/rex && \
                            cp /usr/share/rex/config.yaml.example ~/.config/rex/config.yaml

If you already have these set up from a prior install, do nothing.

Launch on the Brick via EmulationStation -> Ports -> Rex, or over ssh:
  ssh brick-root '/userdata/system/apps/rex/start-rex.sh'
EOF

exit 0
