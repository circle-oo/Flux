#!/usr/bin/env bash
set -euo pipefail

# Flux Startup Script
# Usage: bash start.sh [--config path/to/config.yaml]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

CONFIG="${1:-config.yaml}"
if [[ "$1" == "--config" ]] && [[ -n "${2:-}" ]]; then
    CONFIG="$2"
fi

# Check binary exists
if [ ! -f go/bin/flux ]; then
    echo "Flux binary not found. Running setup first..."
    bash setup.sh
    echo ""
fi

# Check config exists
if [ ! -f "$CONFIG" ]; then
    echo "ERROR: Config file not found: $CONFIG"
    echo ""
    if [ -f config.yaml.template ]; then
        echo "Create one from the template:"
        echo "  cp config.yaml.template config.yaml"
        echo "  # Edit config.yaml with your settings"
    fi
    exit 1
fi

# Check required env vars from config
check_env() {
    local var_name="$1"
    local env_key
    env_key=$(grep "$var_name" "$CONFIG" 2>/dev/null | grep -oE '"[^"]*"' | tr -d '"' | head -1)
    if [ -n "$env_key" ] && [ -z "${!env_key:-}" ]; then
        echo "WARNING: Environment variable $env_key is not set (from $var_name)"
        return 1
    fi
    return 0
}

WARNINGS=0
check_env "password_env" || WARNINGS=$((WARNINGS + 1))
check_env "token_env" || WARNINGS=$((WARNINGS + 1))

if [ "$WARNINGS" -gt 0 ]; then
    echo ""
    echo "Set required variables before starting:"
    echo "  export FLUX_UI_PASSWORD='your-password'"
    echo "  export GITHUB_TOKEN='your-token'  (optional)"
    echo ""
    read -rp "Continue anyway? [y/N] " answer
    if [[ ! "$answer" =~ ^[Yy] ]]; then
        exit 1
    fi
fi

# Create runtime directories
mkdir -p data logs

# Start Flux
echo "Starting Flux..."
echo "  Config: $CONFIG"
echo "  Web UI: http://localhost:$(grep -E '^\s+port:' "$CONFIG" | grep -oE '[0-9]+' | head -1 || echo 8080)"
echo "  Logs:   $(grep -E '^\s+file:' "$CONFIG" | grep -oE '"[^"]*"' | tr -d '"' | head -1 || echo './logs/flux.log')"
echo ""
echo "Press Ctrl+C to stop."
echo ""

exec ./go/bin/flux --config "$CONFIG"
