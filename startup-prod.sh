#!/usr/bin/env bash
set -euo pipefail

# Flux Production Startup
# Full build + install as launchd service (auto-restart, run at login).
# Usage: bash startup-prod.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

PLIST_NAME="com.circle-oo.flux"
PLIST_SRC="$SCRIPT_DIR/deploy/${PLIST_NAME}.plist"
PLIST_DST="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"
CONFIG="$SCRIPT_DIR/config.yaml"

echo "=== Flux Production Deploy ==="
echo ""

# ── Preflight checks ────────────────────────────────────────────────
if [ ! -f "$CONFIG" ]; then
    echo "ERROR: config.yaml not found"
    echo "  Run: bash setup.sh"
    exit 1
fi

if [ ! -f "$PLIST_SRC" ]; then
    echo "ERROR: Launchd plist not found at $PLIST_SRC"
    exit 1
fi

# ── Check required env vars ─────────────────────────────────────────
echo "Checking environment..."

REQUIRED_VARS=(FLUX_UI_PASSWORD GITHUB_TOKEN)
MISSING=()

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var:-}" ]; then
        MISSING+=("$var")
    else
        echo "  $var: set"
    fi
done

if [ ${#MISSING[@]} -gt 0 ]; then
    echo ""
    echo "ERROR: Required environment variables not set:"
    for var in "${MISSING[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "Add them to ~/.zshenv so launchd can pick them up:"
    echo "  export FLUX_UI_PASSWORD='your-password'"
    echo "  export GITHUB_TOKEN='your-token'"
    exit 1
fi

# ── Persist env vars to ~/.zshenv (for launchd) ─────────────────────
ZSHENV="$HOME/.zshenv"
VARS_TO_PERSIST=(FLUX_UI_PASSWORD GITHUB_TOKEN DISCORD_WEBHOOK_URL)

for var in "${VARS_TO_PERSIST[@]}"; do
    val="${!var:-}"
    [ -z "$val" ] && continue
    if [ -f "$ZSHENV" ] && grep -q "^export $var=" "$ZSHENV" 2>/dev/null; then
        continue
    fi
    echo "export $var='$val'" >> "$ZSHENV"
    echo "  Persisted $var to ~/.zshenv"
done

# ── Stop existing service if running ────────────────────────────────
if launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    echo ""
    echo "Stopping existing service..."
    launchctl unload "$PLIST_DST" 2>/dev/null || true
    sleep 1
fi

# ── Build ────────────────────────────────────────────────────────────
echo ""
echo "Building..."
mkdir -p data logs
make build
echo ""

# ── Install and start launchd service ────────────────────────────────
echo "Installing launchd service..."
cp "$PLIST_SRC" "$PLIST_DST"
launchctl load "$PLIST_DST"

# ── Verify ───────────────────────────────────────────────────────────
sleep 2
if launchctl list 2>/dev/null | grep -q "$PLIST_NAME"; then
    PID=$(launchctl list | grep "$PLIST_NAME" | awk '{print $1}')
    EXIT_CODE=$(launchctl list | grep "$PLIST_NAME" | awk '{print $2}')
    if [ "$PID" != "-" ] && [ "${EXIT_CODE:-0}" = "0" ]; then
        echo ""
        echo "Flux is running (PID: $PID)"
    else
        echo ""
        echo "WARNING: Flux may have failed to start (exit: $EXIT_CODE)"
        echo "  Check: tail -20 logs/flux-stderr.log"
    fi
else
    echo ""
    echo "WARNING: Service not found after install"
fi

PORT=$(grep -E '^\s+port:' "$CONFIG" | grep -oE '[0-9]+' | head -1 || echo 8080)

echo ""
echo "  Web UI:  http://localhost:$PORT"
echo "  Binary:  $SCRIPT_DIR/go/bin/flux"
echo "  Config:  $CONFIG"
echo "  Logs:    $SCRIPT_DIR/logs/flux.log"
echo ""
echo "Commands:"
echo "  Status:   launchctl list | grep flux"
echo "  Logs:     tail -f logs/flux.log"
echo "  Stop:     bash stop-prod.sh"
echo "  Restart:  bash stop-prod.sh && bash startup-prod.sh"
