#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_DIR"

# Build flux binary via Makefile
echo "Building flux..."
make build

# Create logs directory
mkdir -p logs

# Generate plist with correct paths
PLIST_NAME="com.circle-oo.flux.plist"
PLIST_SRC="$REPO_DIR/deploy/$PLIST_NAME"
PLIST_DST="$HOME/Library/LaunchAgents/$PLIST_NAME"

# Unload existing service if present
if launchctl list 2>/dev/null | grep -q "com.circle-oo.flux"; then
    echo "Stopping existing service..."
    launchctl unload "$PLIST_DST" 2>/dev/null || true
fi

# Install launchd plist
echo "Installing launchd plist..."
cp "$PLIST_SRC" "$PLIST_DST"
launchctl load "$PLIST_DST"

echo ""
echo "Flux installed and started via launchd."
echo "  Binary:  $REPO_DIR/go/bin/flux"
echo "  Config:  $REPO_DIR/config.yaml"
echo "  Logs:    $REPO_DIR/logs/flux-stdout.log"
echo ""
echo "Commands:"
echo "  Check status:  launchctl list | grep flux"
echo "  View logs:     tail -f logs/flux-stdout.log"
echo "  Stop:          launchctl unload ~/Library/LaunchAgents/$PLIST_NAME"
echo "  Restart:       launchctl unload ~/Library/LaunchAgents/$PLIST_NAME && launchctl load ~/Library/LaunchAgents/$PLIST_NAME"
