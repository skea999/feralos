#!/usr/bin/env bash
# docs_check.sh — documentation gate (P0-3, covering the P3-6 doc checks
# that do not need external tooling):
#   1. relative markdown link check across the project docs
#   2. docs/ orphan check vs the indexes (WARN only — orphan cleanup is
#      tracked as P3-5; failing on it comes later)
#   3. AGENTS.md <-> docs drift SPOT check (D-Bus Activation Rule amended R1
#      + FirstBoot webmin) — fails on stale or missing markers
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BROKEN=0

fail() { echo "✗ $1" >&2; exit 1; }

# --- collect project markdown files ---------------------------------------
# Scan scope: every *.md except VCS/tooling dirs (top-level dot-dirs such as
# .git/.agents/.opencode), references/ (analysis material, not docs) and
# tests/artifacts (gitignored run output, absent from the docker context).
mapfile -t MD_FILES < <(
  find . \( -type d -path './.*' -o -type d -path './references' \
    -o -type d -path './tests/artifacts' \) -prune \
    -o -type f -name '*.md' -print | sort
)

echo "=== Docs: relative link check (${#MD_FILES[@]} markdown files) ==="
for f in "${MD_FILES[@]}"; do
  dir=$(dirname "$f")
  # markdown link targets: ](target) — skip empty, pure anchors and external
  while IFS= read -r target; do
    [ -n "$target" ] || continue
    case "$target" in
      '#'*) continue ;;          # in-page anchor
      http://* | https://* | mailto:*) continue ;; # external
    esac
    # strip optional <...> wrapping and any #fragment
    target="${target#<}"
    target="${target%>}"
    target="${target%%#*}"
    [ -n "$target" ] || continue
    # link targets into excluded scopes are not checked (not part of the
    # docs tree; also absent from the docker build context)
    first="${target%%/*}"
    case "$first" in
      .* | references) continue ;;
    esac
    if [ ! -e "$dir/$target" ]; then
      echo "BROKEN LINK: $f -> $target"
      BROKEN=1
    fi
  done < <(grep -oE '\]\([^)]+\)' "$f" | sed -e 's/^\](//' -e 's/)$//' | cut -d'#' -f1)
done
[ "$BROKEN" -eq 0 ] || fail "broken relative markdown links found"
echo "✓ relative links OK"

# --- docs/ orphan check (WARN only) ----------------------------------------
echo "=== Docs: orphan check (WARN only, cleanup tracked as P3-5) ==="
# Indexes = the pointers AGENTS.md keeps + root README.md + every README
# under docs/. A docs file counts as reachable when its repo-relative path
# or its basename appears in any index text.
mapfile -t INDEXES < <(printf '%s\n' AGENTS.md README.md; find docs -type f -name 'README.md')
INDEX_TEXT="$(cat "${INDEXES[@]}")"
mapfile -t FRAGMENTS < <(grep -oE '[A-Za-z0-9_./-]+\.md' <<<"$INDEX_TEXT" | sort -u)

ORPHANS=0
for f in "${MD_FILES[@]}"; do
  rel="${f#./}"
  case "$rel" in docs/*) ;; *) continue ;; esac # orphan check scope: docs/ only
  base=$(basename "$rel")
  reachable=0
  for frag in "${FRAGMENTS[@]}"; do
    if [ "$frag" = "$rel" ] || [ "${frag##*/}" = "$base" ]; then
      reachable=1
      break
    fi
  done
  if [ "$reachable" -eq 0 ]; then
    echo "WARN: orphan (not indexed): $rel"
    ORPHANS=$((ORPHANS + 1))
  fi
done
echo "orphan check: $ORPHANS unindexed docs/ files (WARN only)"

# --- AGENTS.md <-> docs drift spot check ------------------------------------
echo "=== Docs: AGENTS.md drift spot check ==="
SECTION_DBUS=$(awk '/^## D-Bus Activation Rule/{f=1; next} /^## /{f=0} f' AGENTS.md)
[ -n "$SECTION_DBUS" ] || fail "AGENTS.md section 'D-Bus Activation Rule' not found"
grep -q "OpenRC initd name" <<<"$SECTION_DBUS" ||
  fail "D-Bus section: missing amended-R1 marker 'OpenRC initd name'"
grep -q "dinitServices" <<<"$SECTION_DBUS" ||
  fail "D-Bus section: missing amended-R1 marker 'dinitServices'"
grep -qiE 'dinitServices.{0,120}(empty|EMPTY)' <<<"$SECTION_DBUS" ||
  fail "D-Bus section: 'dinitServices' not documented as empty/EMPTY"
if grep -q "enables dinit units whose names differ" <<<"$SECTION_DBUS"; then
  fail "D-Bus section: STALE pre-amendment phrasing 'enables dinit units whose names differ'"
fi

SECTION_FB=$(awk '/^## FirstBoot Architecture$/{f=1; next} /^## /{f=0} f' AGENTS.md)
[ -n "$SECTION_FB" ] || fail "AGENTS.md section 'FirstBoot Architecture' not found"
grep -q "feralos_webmin" <<<"$SECTION_FB" ||
  fail "FirstBoot section: 'feralos_webmin' missing from the 3-boot flow"
echo "✓ AGENTS.md drift spot check passed (D-Bus R1 + FirstBoot webmin)"

echo "=== Docs check complete ==="
exit 0
