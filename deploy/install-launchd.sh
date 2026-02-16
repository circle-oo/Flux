#!/bin/zsh
# Install launchd plists for Flux Go backend and Python Agent Manager.
# Usage: ./deploy/install-launchd.sh [--uninstall]

set -euo pipefail

PLIST_DIR="$HOME/Library/LaunchAgents"
DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"

FLUX_PLIST="com.circle-oo.flux.plist"
AGENT_PLIST="com.circle-oo.flux-agent.plist"

uninstall() {
    echo "Uninstalling Flux launchd services..."
    launchctl bootout "gui/$(id -u)/$FLUX_PLIST" 2>/dev/null || true
    launchctl bootout "gui/$(id -u)/$AGENT_PLIST" 2>/dev/null || true
    rm -f "$PLIST_DIR/$FLUX_PLIST" "$PLIST_DIR/$AGENT_PLIST"
    echo "Done."
}

install() {
    echo "Installing Flux launchd services..."

    # Ensure log directory exists
    mkdir -p /Users/won.park/workspace/flux/logs

    # Make wrapper scripts executable
    chmod +x "$DEPLOY_DIR/run-flux.sh"
    chmod +x "$DEPLOY_DIR/run-flux-agent.sh"

    # Stop existing services if running
    launchctl bootout "gui/$(id -u)/$FLUX_PLIST" 2>/dev/null || true
    launchctl bootout "gui/$(id -u)/$AGENT_PLIST" 2>/dev/null || true

    # Copy plists
    cp "$DEPLOY_DIR/$FLUX_PLIST" "$PLIST_DIR/$FLUX_PLIST"
    cp "$DEPLOY_DIR/$AGENT_PLIST" "$PLIST_DIR/$AGENT_PLIST"

    # Start Python Agent Manager first (Go backend depends on it)
    echo "Starting Python Agent Manager (port 50051)..."
    launchctl bootstrap "gui/$(id -u)" "$PLIST_DIR/$AGENT_PLIST"

    # Brief wait for Python to bind its port
    sleep 2

    # Start Go backend
    echo "Starting Go backend (port 8080)..."
    launchctl bootstrap "gui/$(id -u)" "$PLIST_DIR/$FLUX_PLIST"

    echo "Done. Services installed and running."
    echo "  Go backend:    launchctl print gui/$(id -u)/com.circle-oo.flux"
    echo "  Python agent:  launchctl print gui/$(id -u)/com.circle-oo.flux-agent"
    echo "  Logs:          /Users/won.park/workspace/flux/logs/"
}

if [[ "${1:-}" == "--uninstall" ]]; then
    uninstall
else
    install
fi
