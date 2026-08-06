# Demo: browse the music library in ncmpcpp, queue an album, play it.
# Runs on the device during a recording; drives the real tmux session.
#
# Starts from a bash prompt with an empty queue. ncmpcpp's browser always
# opens at the music root, so the navigation below is deterministic.
mpc clear >/dev/null 2>&1
mpc stop  >/dev/null 2>&1

key() { tmux send-keys -t rex "$1"; sleep "${2:-0.9}"; }

sleep 1
tmux send-keys -t rex "ncmpcpp" Enter
sleep 3.5

key 2 2          # browser
key Enter 2      # into soundcloud/
key Down
key Enter 2.5    # into first artist

# The cursor opens on [..], where Space is a no-op — step onto the first
# track before queueing. Space queues and advances the cursor.
key Down
key Space
key Space
key Space
sleep 1

key 1 2          # back to the playlist, now three tracks deep
key Enter 7      # play, and let the progress bar run

tmux send-keys -t rex q
