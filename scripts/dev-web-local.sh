#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir/web"

: "${DEV_WEB_RUNTIME:=auto}"
case "$DEV_WEB_RUNTIME" in
  auto)
    if command -v bun >/dev/null 2>&1; then
      DEV_WEB_RUNTIME=bun
    elif command -v npm >/dev/null 2>&1; then
      DEV_WEB_RUNTIME=npm
    else
      echo "bun or npm is required; install Bun with 'brew install bun' or Node.js/npm" >&2
      exit 1
    fi
    ;;
  bun|npm) ;;
  *)
    echo "DEV_WEB_RUNTIME must be auto, bun, or npm" >&2
    exit 1
    ;;
esac

if [ ! -d node_modules ]; then
  if [ "$DEV_WEB_RUNTIME" = bun ]; then
    bun install --frozen-lockfile
  else
    npm install --no-package-lock
  fi
fi

: "${DEV_WEB_PORT:=5173}"
: "${DEV_API_URL:=http://127.0.0.1:3100}"
echo "new-api frontend dev server: http://127.0.0.1:${DEV_WEB_PORT}"
echo "API proxy target: ${DEV_API_URL} (${DEV_WEB_RUNTIME})"
export VITE_REACT_APP_SERVER_URL="${VITE_REACT_APP_SERVER_URL:-$DEV_API_URL}"
if [ "$DEV_WEB_RUNTIME" = bun ]; then
  exec bun run dev -- --host 0.0.0.0 --port "${DEV_WEB_PORT}"
fi
exec npm run dev -- --host 0.0.0.0 --port "${DEV_WEB_PORT}"
