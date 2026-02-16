#!/usr/bin/env bash
set -euo pipefail

# Flux Production Stop
# Unloads the launchd service (stops the process and prevents auto-restart).
# Usage: bash stop-prod.sh

PLIST_NAME="com.circle-oo.flux"
PLIST_DST="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"

echo "=== Flux Stop ==="
echo ""

# ── Check if service exists ──────────────────────────────────────────
if ! launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    echo "Flux service is not loaded."

    # Clean up stale plist if it exists on disk
    if [ -f "$PLIST_DST" ]; then
        echo "Removing stale plist: $PLIST_DST"
        rm -f "$PLIST_DST"
    fi

    exit 0
fi

# ── Show current status ─────────────────────────────────────────────
PID=$(launchctl list | grep "$PLIST_NAME" | awk '{print $1}')
if [ "$PID" != "-" ]; then
    echo "Flux is running (PID: $PID)"
else
    echo "Flux is loaded but not running"
fi

# ── Unload service ───────────────────────────────────────────────────
echo "Unloading launchd service..."
launchctl unload "$PLIST_DST" 2>/dev/null || true

# ── Verify ───────────────────────────────────────────────────────────
sleep 1
if launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    echo "WARNING: Service still appears loaded. Trying force removal..."
    launchctl remove "$PLIST_NAME" 2>/dev/null || true
    sleep 1
fi

if ! launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    echo "Flux stopped."
else
    echo "ERROR: Failed to stop Flux. Manual cleanup:"
    echo "  launchctl unload $PLIST_DST"
    echo "  launchctl remove $PLIST_NAME"
    exit 1
fi
