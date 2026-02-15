#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_DIR"

# ── Check required environment variables ──────────────────────────────
echo "Checking environment variables..."

REQUIRED_VARS=(FLUX_UI_PASSWORD GITHUB_TOKEN)
OPTIONAL_VARS=(DISCORD_WEBHOOK_URL)
MISSING=()

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING+=("$var")
    fi
done

if [ ${#MISSING[@]} -gt 0 ]; then
    echo ""
    echo "ERROR: Required environment variables are not set:"
    for var in "${MISSING[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "Add them to ~/.zshenv so launchd can pick them up:"
    echo ""
    echo "  cat >> ~/.zshenv << 'EOF'"
    for var in "${REQUIRED_VARS[@]}"; do
        echo "  export $var='your-value-here'"
    done
    for var in "${OPTIONAL_VARS[@]}"; do
        echo "  export $var='your-value-here'  # optional"
    done
    echo "  EOF"
    echo ""
    echo "Then open a new terminal and re-run this script."
    exit 1
fi

echo "  FLUX_UI_PASSWORD: set"
echo "  GITHUB_TOKEN: set"
for var in "${OPTIONAL_VARS[@]}"; do
    if [ -n "${!var}" ]; then
        echo "  $var: set"
    else
        echo "  $var: not set (optional)"
    fi
done

# ── Ensure ~/.zshenv has the env vars ─────────────────────────────────
ZSHENV="$HOME/.zshenv"
VARS_TO_PERSIST=(FLUX_UI_PASSWORD GITHUB_TOKEN DISCORD_WEBHOOK_URL)
ADDED_VARS=()

for var in "${VARS_TO_PERSIST[@]}"; do
    val="${!var}"
    if [ -z "$val" ]; then
        continue
    fi
    if [ -f "$ZSHENV" ] && grep -q "^export $var=" "$ZSHENV" 2>/dev/null; then
        continue  # already in .zshenv
    fi
    echo "export $var='$val'" >> "$ZSHENV"
    ADDED_VARS+=("$var")
done

if [ ${#ADDED_VARS[@]} -gt 0 ]; then
    echo ""
    echo "Added to ~/.zshenv: ${ADDED_VARS[*]}"
fi

# ── Build flux binary ─────────────────────────────────────────────────
echo ""
echo "Building flux..."
make build

# Create logs directory
mkdir -p logs

# ── Install launchd service ───────────────────────────────────────────
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

# ── Verify ────────────────────────────────────────────────────────────
sleep 2
if launchctl list 2>/dev/null | grep -q "com.circle-oo.flux"; then
    PID=$(launchctl list | grep com.circle-oo.flux | awk '{print $1}')
    EXIT=$(launchctl list | grep com.circle-oo.flux | awk '{print $2}')
    if [ "$EXIT" = "0" ] && [ "$PID" != "-" ]; then
        echo ""
        echo "Flux installed and running (PID: $PID)"
    else
        echo ""
        echo "WARNING: Flux may have failed to start (exit: $EXIT)"
        echo "Check logs: tail -20 $REPO_DIR/logs/flux-stderr.log"
    fi
else
    echo ""
    echo "WARNING: Flux service not found after install"
fi

echo ""
echo "  Binary:   $REPO_DIR/go/bin/flux"
echo "  Config:   $REPO_DIR/config.yaml"
echo "  Wrapper:  $REPO_DIR/deploy/run-flux.sh"
echo "  Logs:     $REPO_DIR/logs/flux.log"
echo "  Env:      ~/.zshenv"
echo ""
echo "Commands:"
echo "  Check status:  launchctl list | grep flux"
echo "  View logs:     tail -f logs/flux.log"
echo "  Stop:          launchctl unload ~/Library/LaunchAgents/$PLIST_NAME"
echo "  Restart:       launchctl unload ~/Library/LaunchAgents/$PLIST_NAME && launchctl load ~/Library/LaunchAgents/$PLIST_NAME"
