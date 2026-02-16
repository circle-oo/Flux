#!/bin/zsh
# Wrapper script for launchd that runs the Python Agent Manager.
# zsh automatically sources ~/.zshenv for env vars.

export PATH="/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin:/opt/homebrew/sbin:$PATH"
export HOME="/Users/won.park"

cd /Users/won.park/workspace/flux

# Use system python3 or homebrew python
PYTHON=$(command -v python3 || echo /opt/homebrew/bin/python3)

exec "$PYTHON" -m agent_manager.server
