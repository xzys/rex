#!/bin/bash

set -euo pipefail

ZOT_VERSION="${ZOT_VERSION:-0.2.93}"

ARCHIVE="zot_${ZOT_VERSION}_linux_arm64.tar.gz"
URL="https://github.com/patriceckhart/zot/releases/download/v${ZOT_VERSION}/${ARCHIVE}"

echo "Downloading zot ${ZOT_VERSION} for linux/arm64..."
curl -fsSL "$URL" -o "/tmp/${ARCHIVE}"

echo "Extracting..."
tar xzf "/tmp/${ARCHIVE}" -C /tmp

cp /tmp/zot /output/zot
chmod +x /output/zot

echo "zot ${ZOT_VERSION} → /output/zot"
