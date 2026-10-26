#!/bin/sh

set -eu

# resolve_path is duplicated in run_mcp.sh —
# keep in sync when editing (dedup intentionally skipped: these are
# convenience scripts against a foreign layout, minimal-risk choice).

resolve_path() {
  if command -v readlink >/dev/null 2>&1; then
    readlink -f "$1"
    return
  fi

  case "$1" in
  /*) printf '%s\n' "$1" ;;
  *) printf '%s/%s\n' "$(pwd -P)" "$1" ;;
  esac
}

script_path=$(resolve_path "$0")
script_dir=$(CDPATH='' cd -- "$(dirname -- "$script_path")" && pwd -P)
repo_root=$(CDPATH='' cd -- "$script_dir/../../.." && pwd -P)
wrapper_path="$repo_root/installer/tools/harness/runMcp.sh"

if [ ! -x "$wrapper_path" ]; then
  echo "error: expected MCP wrapper at $wrapper_path" >&2
  exit 1
fi

cat <<EOF
{
  "servers": {
    "skea-harness": {
      "type": "stdio",
      "command": "$wrapper_path"
    }
  }
}
EOF
