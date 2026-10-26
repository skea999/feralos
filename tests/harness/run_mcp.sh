#!/bin/sh

set -eu

# resolve_path is duplicated in generate_mcp_config.sh —
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
binary_path="$repo_root/artifacts/harness/bin/skea-harness"
artifacts_root="$repo_root/artifacts/harness"
harness_src_dir="$repo_root/installer/tools/harness"

binary_needs_rebuild() {
	if [ ! -x "$binary_path" ]; then
		return 0
	fi

	find "$harness_src_dir" \
		-type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) \
		-newer "$binary_path" | grep -q .
}

if binary_needs_rebuild; then
	mkdir -p "$(dirname -- "$binary_path")"
	echo "info: building harness binary at $binary_path" >&2
	(
		cd "$harness_src_dir"
		go build -o "$binary_path" ./cmd/skea_harness
	)
fi

exec "$binary_path" mcp --repo-root "$repo_root" --artifacts-root "$artifacts_root" "$@"
