#!/bin/bash
# Install Claude Code via npm.
# Requires Node.js 18+ (Node 20 recommended).
set -e

echo "installing claude-code (this may take a minute)..."
npm install -g @anthropic-ai/claude-code

echo
echo "claude-code installed"
echo "press enter to close..."
read -r _
