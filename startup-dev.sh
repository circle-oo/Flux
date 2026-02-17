#!/usr/bin/env bash
set -euo pipefail

# Flux Development Startup
# Builds frontend + backend with hot output, runs in foreground.
# Also starts the Python Agent Manager as a background process.
# Usage: bash startup-dev.sh [--skip-frontend]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

SKIP_FRONTEND=false
if [[ "${1:-}" == "--skip-frontend" ]]; then
    SKIP_FRONTEND=true
fi

CONFIG="$SCRIPT_DIR/config.yaml"
if [ ! -f "$CONFIG" ]; then
    echo "ERROR: config.yaml not found"
    echo "  Run: bash setup.sh"
    exit 1
fi

# ── Prerequisites ────────────────────────────────────────────────────
echo "=== Flux Dev ==="
echo ""

if ! command -v go &>/dev/null; then
    echo "ERROR: Go not found. Install with: brew install go"
    exit 1
fi

if ! command -v node &>/dev/null; then
    echo "ERROR: Node not found. Install with: brew install node"
    exit 1
fi

if ! command -v python3 &>/dev/null; then
    echo "ERROR: Python3 not found. Install with: brew install python"
    exit 1
fi

# ── Generate protobuf stubs ──────────────────────────────────────────
if command -v buf &>/dev/null; then
    echo "Generating protobuf stubs..."
    make proto 2>&1 | tail -1
    echo ""
else
    echo "WARNING: buf not found, skipping proto generation."
    echo "  Install with: brew install bufbuild/buf/buf"
    echo ""
fi

# ── Build frontend ───────────────────────────────────────────────────
if [ "$SKIP_FRONTEND" = false ]; then
    echo "Building frontend..."
    cd frontend && npm install --silent && npm run build 2>&1 | tail -3
    cd "$SCRIPT_DIR"

    echo "Embedding frontend..."
    rm -rf go/src/web/dist
    cp -r frontend/dist go/src/web/dist
    echo ""
fi

# ── Start Python Agent Manager in background ─────────────────────────
AGENT_PID=""
cleanup() {
    if [ -n "$AGENT_PID" ] && kill -0 "$AGENT_PID" 2>/dev/null; then
        echo ""
        echo "Stopping Agent Manager (PID: $AGENT_PID)..."
        kill "$AGENT_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "Starting Agent Manager..."
python3 -m agent_manager.server &
AGENT_PID=$!
sleep 1

if kill -0 "$AGENT_PID" 2>/dev/null; then
    echo "  Agent Manager running (PID: $AGENT_PID)"
else
    echo "WARNING: Agent Manager failed to start. Continuing without it."
    AGENT_PID=""
fi
echo ""

# ── Run backend (go run, no binary needed) ───────────────────────────
PORT=$(grep -E '^\s+port:' "$CONFIG" | grep -oE '[0-9]+' | head -1 || echo 8080)

echo "Starting Flux (dev mode)..."
echo "  Config:  $CONFIG"
echo "  Web UI:  http://localhost:$PORT"
echo ""
echo "Press Ctrl+C to stop."
echo ""

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
exec go run -C go/src -ldflags "-X main.version=${VERSION}-dev" ./cmd/flux --config "$CONFIG"
