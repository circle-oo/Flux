#!/bin/zsh
# Wrapper script for launchd that runs flux with the user's zsh environment.
# zsh automatically sources ~/.zshenv for env vars (FLUX_UI_PASSWORD, etc.)
# PATH additions needed for node/npm/claude and go tools.

export PATH="/usr/local/Cellar/node/25.5.0/bin:/usr/local/go/bin:/Users/won.park/go/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:$PATH"

exec /Users/won.park/workspace/flux/go/bin/flux --config /Users/won.park/workspace/flux/config.yaml
