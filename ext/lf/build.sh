#!/bin/bash

set -e

go install -ldflags="-s -w" github.com/gokcehan/lf@latest
cp /root/go/bin/linux_arm64/lf /output/lf
