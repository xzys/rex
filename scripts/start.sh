#!/bin/bash
# start tmux and server, exit server when tmux exits

set -e

# Kill any existing server processes
pkill rex 2>/dev/null || true

# Check if tmux session already exists
TMUX_SESSION="rex"
if tmux has-session -t "$TMUX_SESSION" 2>/dev/null; then
    echo "Tmux session $TMUX_SESSION already exists"
    exit 1
fi

# Start server in background
/home/ark/bin/rex server &
SERVER_PID=$!

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up server..."
    kill $SERVER_PID 2>/dev/null || true
}

trap cleanup EXIT

# runs in foreground, blocks until tmux exits or detaches
tmux new-session -s "$TMUX_SESSION"