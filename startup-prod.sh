#!/usr/bin/env bash
set -euo pipefail

# Flux Production Startup
# Builds Go + frontend, then installs launchd services (Go backend + Python Agent Manager).
# Usage: bash startup-prod.sh [--backend-only]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

CONFIG="$SCRIPT_DIR/config.yaml"

echo "=== Flux Production Deploy ==="
echo ""

# ── Preflight checks ────────────────────────────────────────────────
if [ ! -f "$CONFIG" ]; then
    echo "ERROR: config.yaml not found"
    echo "  Run: bash setup.sh"
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

# ── Build ────────────────────────────────────────────────────────────
echo ""
echo "Building..."
mkdir -p data logs

if [[ "${1:-}" == "--backend-only" ]]; then
    make build-backend
else
    make build
fi
echo ""

# ── Install and start launchd services ───────────────────────────────
echo "Installing launchd services..."
make install-services

# ── Verify ───────────────────────────────────────────────────────────
sleep 2
PORT=$(grep -E '^\s+port:' "$CONFIG" | grep -oE '[0-9]+' | head -1 || echo 8080)

echo ""
echo "  Web UI:  http://localhost:$PORT"
echo "  Binary:  $SCRIPT_DIR/go/bin/flux"
echo "  Config:  $CONFIG"
echo "  Logs:    $SCRIPT_DIR/logs/"
echo ""
echo "Commands:"
echo "  Status:   launchctl list | grep flux"
echo "  Logs:     tail -f logs/flux.log"
echo "  Stop:     bash stop-prod.sh"
echo "  Restart:  bash stop-prod.sh && bash startup-prod.sh"
