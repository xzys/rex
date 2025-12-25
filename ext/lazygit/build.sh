#!/bin/bash

set -e

go install -ldflags="-s -w" github.com/jesseduffield/lazygit@latest
cp /root/go/bin/linux_arm64/lazygit /output/lazygit

