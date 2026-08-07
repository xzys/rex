# Demo: open the rex menu with Start, launch ncmpcpp from it, browse the
# library and play a track — driven entirely through the gamepad.
#
# Presses go to rex's control socket, which hands them to the same operator
# the physical buttons feed, so this exercises the real input path (menu
# navigation, the per-app ncmpcpp mapping, the global fallback) rather than
# typing commands into a shell.
#
# Runs on the device during a recording.

. /tmp/rex-demo-lib.sh

# Back to a single shell window with an empty queue, so the demo starts from
# the same place every take.
reset_windows
mpc clear >/dev/null 2>&1
mpc stop  >/dev/null 2>&1
sleep 1.5

pad start 2.5      # Start opens the menu in its own window

# The menu is a 4-wide grid; ncmpcpp is the fourth tile on the top row.
pad right 0.7
pad right 0.7
pad right 1.2
pad a 5            # the tile's wrapper starts mpd, then execs ncmpcpp

# In ncmpcpp the d-pad cycles screens: right steps to Browser, and the
# matching left at the end steps back to Playlist. Keeping the two balanced
# leaves the plugin's screen counter where the next take expects it.
pad right 2        # Browser
pad a 2            # into soundcloud/
pad down
pad a 2.5          # into the artist directory
pad down           # step off [..] onto the first track
pad a 3            # A is Enter: queues the track and starts playback
pad left 2         # back to Playlist
sleep 7            # let the progress bar run

pad start 3        # Start again — back to the menu, how you leave an app
