#!/usr/bin/env bash
set -euo pipefail

# Flux Setup Script — first-time dependency install + build.
# Usage: bash setup.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== Flux Setup ==="
echo ""

# 1. Check prerequisites
echo "Checking prerequisites..."

if ! command -v go &>/dev/null; then
    echo "ERROR: Go is not installed. Please install Go 1.22+."
    echo "  brew install go"
    exit 1
fi

GO_VERSION=$(go version | grep -oE '[0-9]+\.[0-9]+' | head -1)
echo "  Go: $GO_VERSION"

if ! command -v node &>/dev/null; then
    echo "ERROR: Node.js is not installed. Please install Node.js 20+."
    echo "  brew install node"
    exit 1
fi

NODE_VERSION=$(node --version)
echo "  Node.js: $NODE_VERSION"

if ! command -v npm &>/dev/null; then
    echo "ERROR: npm is not installed."
    exit 1
fi

echo "  npm: $(npm --version)"

if ! command -v notesmd-cli &>/dev/null; then
    echo ""
    echo "Installing notesmd-cli..."
    brew install yakitrak/yakitrak/notesmd-cli
fi
echo "  notesmd-cli: $(notesmd-cli --version 2>/dev/null || echo 'installed')"
echo ""

DEFAULT_CONFIG="$SCRIPT_DIR/config.yaml"
TEMPLATE_CONFIG="$SCRIPT_DIR/config.yaml.template"

# 2. Create config from template
if [ ! -f "$DEFAULT_CONFIG" ]; then
    if [ -f "$TEMPLATE_CONFIG" ]; then
        echo "Creating config.yaml from template..."
        cp "$TEMPLATE_CONFIG" "$DEFAULT_CONFIG"
        echo "  Created: $DEFAULT_CONFIG"
        echo "  IMPORTANT: Edit config.yaml and set your environment variables:"
        echo "    export FLUX_UI_PASSWORD='your-password'"
        echo "    export GITHUB_TOKEN='your-github-token'"
        echo "    export DISCORD_WEBHOOK_URL='your-webhook-url' (optional)"
        echo ""
    else
        echo "WARNING: No config.yaml or config.yaml.template found."
        echo "  Create config.yaml before running Flux."
        echo ""
    fi
else
    echo "config.yaml already exists, skipping."
    echo ""
fi

# 3. Install frontend dependencies
echo "Installing frontend dependencies..."
cd frontend
npm install --silent 2>&1 | tail -1
cd "$SCRIPT_DIR"
echo "  Done."
echo ""

# 4. Build frontend
echo "Building frontend..."
cd frontend
npm run build 2>&1 | tail -3
cd "$SCRIPT_DIR"
echo "  Done."
echo ""

# 5. Embed frontend into Go
echo "Embedding frontend..."
rm -rf go/src/web/dist
cp -r frontend/dist go/src/web/dist
echo "  Done."
echo ""

# 6. Fetch Go dependencies
echo "Fetching Go dependencies..."
cd go/src
go mod download 2>&1
cd "$SCRIPT_DIR"
echo "  Done."
echo ""

# 7. Build Go binary
echo "Building Flux binary..."
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
cd go/src
go build -ldflags "-X main.version=$VERSION" -o ../bin/flux ./cmd/flux
cd "$SCRIPT_DIR"
echo "  Built: go/bin/flux"
echo ""

# 8. Create runtime directories
mkdir -p data logs
echo "Created runtime directories (data/, logs/)."
echo ""

# 9. Register Obsidian vault with notesmd-cli
if command -v notesmd-cli &>/dev/null; then
    echo "Registering Obsidian vault with notesmd-cli..."
    notesmd-cli set-default "Flux" 2>/dev/null || echo "  NOTE: Open the Flux vault in Obsidian at least once, then re-run setup."
    echo "  Done."
    echo ""
fi

echo "=== Setup Complete ==="
echo ""
echo "Before starting, set required environment variables:"
echo "  export FLUX_UI_PASSWORD='your-password'"
echo ""
echo "To start Flux:"
echo "  bash startup-dev.sh           # dev — foreground, go run"
echo "  bash startup-prod.sh          # prod — launchd service"
echo ""
echo "To stop production:"
echo "  bash stop-prod.sh"
echo ""
echo "Then open http://localhost:8080 in your browser."
