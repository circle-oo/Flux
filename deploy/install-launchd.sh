#!/bin/bash
set -e

# Build flux binary
cd "$(dirname "$0")/.."
echo "Building flux..."
cd go/src && go build -o ../../flux ./cmd/flux && cd ../..

# Copy to /usr/local/bin
echo "Installing binary..."
sudo cp flux /usr/local/bin/flux

# Create logs directory
mkdir -p logs

# Install launchd plist
echo "Installing launchd plist..."
cp deploy/com.circle-oo.flux.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.circle-oo.flux.plist

echo "Flux installed and started via launchd."
echo "Check status: launchctl list | grep flux"
echo "View logs: tail -f logs/flux-stdout.log"
