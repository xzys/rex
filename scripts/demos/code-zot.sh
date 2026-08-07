# Demo: open the zot coding agent from the menu and ask it what it is running
# on — the agent inspects the handheld and answers from what it finds.
. /tmp/rex-demo-lib.sh

reset_windows
sleep 1.5

pad start 2.5      # Start opens the menu

# Top row is shell, pi, zot, ncmpcpp.
pad right 0.7
pad right 1.2
pad a 7            # zot's TUI

type_text "what hardware and OS are you running on?"
sleep 0.8
tmux send-keys -t rex Enter
sleep 16           # the agent pokes around the system, then answers

pad start 3        # back to the menu
