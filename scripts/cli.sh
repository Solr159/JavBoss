#!/usr/bin/env bash
set -euo pipefail

trap 'printf "\e[?25h"' EXIT

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_ROOT="$SCRIPT_DIR/cli"
CLI_BIN="$CLI_ROOT/build/javboss-cli.cjs"
NEED_BUILD=0

if [[ ! -f "$CLI_BIN" ]]; then
  NEED_BUILD=1
else
  if find "$CLI_ROOT" -type f \( -name "*.mjs" -o -name "*.js" -o -name "*.json" \) \
    ! -path "$CLI_ROOT/node_modules/*" ! -path "$CLI_ROOT/build/*" \
    -newer "$CLI_BIN" -print -quit | grep -q .; then
    NEED_BUILD=1
  fi
fi

if [[ "$NEED_BUILD" == "1" ]]; then
  echo "bundled CLI missing or stale; building..." >&2
  pushd "$CLI_ROOT" >/dev/null
  if [[ ! -d node_modules ]]; then
    if ! command -v npm >/dev/null 2>&1; then
      echo "npm not found; please install Node.js/npm to build CLI" >&2
      popd >/dev/null
      exit 1
    fi
    npm install
  fi
  npm run build
  popd >/dev/null
fi

node "$CLI_BIN" "$@"
