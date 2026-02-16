#!/usr/bin/env bash
set -euo pipefail

# Flux Production Stop
# Unloads all launchd services (Go backend + Python Agent Manager).
# Usage: bash stop-prod.sh

echo "=== Flux Stop ==="
echo ""

make uninstall-services
