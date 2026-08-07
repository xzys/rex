# Demo: open a shell on another machine over SSH, from the handheld.
#
# The ssh tile runs `rex menu --ssh`: pick a host from ~/.ssh/config, then the
# app menu reopens in "remote" mode, so any tile on it runs over there instead
# of locally. Here that's a plain shell on an Armbian box.
. /tmp/rex-demo-lib.sh

reset_windows
sleep 1.5

pad start 2.5      # Start opens the menu

# Second row is lazygit, lf, games, ssh.
pad down 0.7
pad right 0.7
pad right 0.7
pad right 1.2
pad a 3            # the host list from ~/.ssh/config

pad a 3            # seaking — the menu reopens, now "open remotely in seaking"
pad a 7            # shell: connect, and let the Armbian splash draw
sleep 4

pad start 3        # back to the menu
