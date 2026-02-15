#!/usr/bin/env bash
set -euo pipefail

# Flux Setup Script
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
echo ""

# 2. Create config from template
if [ ! -f config.yaml ]; then
    echo "Creating config.yaml from template..."
    cp config.yaml.template config.yaml
    echo "  Created config.yaml"
    echo "  IMPORTANT: Edit config.yaml and set your environment variables:"
    echo "    export FLUX_UI_PASSWORD='your-password'"
    echo "    export GITHUB_TOKEN='your-github-token'"
    echo "    export DISCORD_WEBHOOK_URL='your-webhook-url' (optional)"
    echo ""
else
    echo "config.yaml already exists, skipping."
    echo ""
fi

# 3. Install frontend dependencies
echo "Installing frontend dependencies..."
cd react/flux-ui
npm install --silent 2>&1 | tail -1
cd "$SCRIPT_DIR"
echo "  Done."
echo ""

# 4. Build frontend
echo "Building frontend..."
cd react/flux-ui
npm run build 2>&1 | tail -3
cd "$SCRIPT_DIR"
echo "  Done."
echo ""

# 5. Embed frontend into Go
echo "Embedding frontend..."
rm -rf go/src/web/dist
cp -r react/flux-ui/dist go/src/web/dist
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

echo "=== Setup Complete ==="
echo ""
echo "Before starting, set required environment variables:"
echo "  export FLUX_UI_PASSWORD='your-password'"
echo ""
echo "To start Flux:"
echo "  ./go/bin/flux --config config.yaml"
echo ""
echo "Then open http://localhost:8080 in your browser."
