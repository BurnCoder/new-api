#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

: "${PORT:=3100}"
: "${SQL_DSN:=postgresql://root:123456@127.0.0.1:55432/new-api}"
: "${REDIS_CONN_STRING:=redis://127.0.0.1:56379}"
: "${TZ:=Asia/Shanghai}"
: "${BATCH_UPDATE_ENABLED:=true}"
: "${SESSION_COOKIE_SECURE:=false}"

export PORT SQL_DSN REDIS_CONN_STRING TZ BATCH_UPDATE_ENABLED SESSION_COOKIE_SECURE
echo "new-api Go dev server: http://127.0.0.1:${PORT}"
echo "database: PostgreSQL (SQL_DSN configured)"
exec go run .
