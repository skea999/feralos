#!/usr/bin/env bash
# docs_check.sh — documentation gate (P0-3, covering the P3-6 doc checks
# that do not need external tooling):
#   1. relative markdown link check across the project docs
#   2. docs/ orphan check vs the indexes (FAIL — since P3-5 every docs/ file
#      is either indexed or deleted; a new unindexed file breaks the gate)
#   3. AGENTS.md <-> docs drift SPOT check (D-Bus Activation Rule amended R1
#      + FirstBoot architecture) — fails on stale or missing markers
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

# --- docs/ orphan check (FAIL) -----------------------------------------------
echo "=== Docs: orphan check ==="
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
    echo "ORPHAN (not indexed): $rel"
    ORPHANS=$((ORPHANS + 1))
  fi
done
if [ "$ORPHANS" -ne 0 ]; then
  fail "$ORPHANS unindexed docs/ file(s) — index them (AGENTS.md Docs Index or the fitting docs/*/README.md) or delete them"
fi
echo "✓ orphan check: 0 unindexed docs/ files"

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
echo "✓ AGENTS.md drift spot check passed (D-Bus R1 + FirstBoot)"

# --- toolchain docs gates (2026-11-13 round) ---------------------------------
echo "=== Docs: toolchain docs gates ==="

# ADR-011 (docs canonical model) exists and is indexed
[ -f docs/adr/ADR-011-docs-canonical-model.md ] ||
  fail "ADR-011-docs-canonical-model.md missing"
grep -q "ADR-011-docs-canonical-model.md" docs/adr/README.md AGENTS.md ||
  fail "ADR-011 not indexed (docs/adr/README.md + AGENTS.md Docs Index)"

# CHANGELOG shape: single H1, [Unreleased] before the first dated entry,
# 'Session work log' present at ## level
[ "$(grep -c '^# ' CHANGELOG.md)" -eq 1 ] || fail "CHANGELOG.md must have exactly one H1"
FIRST_DATED=$(awk '/^## 2026-/{print NR; exit}' CHANGELOG.md)
UNREL=$(awk '/^## \[Unreleased\]/{print NR; exit}' CHANGELOG.md)
[ -n "$FIRST_DATED" ] && [ -n "$UNREL" ] && [ "$UNREL" -lt "$FIRST_DATED" ] ||
  fail "CHANGELOG.md: '## [Unreleased]' must precede the first dated '## 2026-' entry"
grep -q '^## Session work log' CHANGELOG.md ||
  fail "CHANGELOG.md: '## Session work log' section missing"

# permission semantics: no first-match claims; denies documented AFTER the
# broad allows (last-match-wins order) in the global-setup examples
if grep -rqnE 'first[- ]match[- ]wins|first match wins' docs/global-setup.md docs/update.md AGENTS.md; then
  fail "stale first-match-wins claim in the toolchain docs (V2 = last-match-wins)"
fi
ORDER_BLOCK=$(awk '/```jsonc/{f=1;next} /^```/{f=0} f' docs/global-setup.md | grep -E '"(action|effect)"')
FIRST_ALLOW=$(awk '/"effect": "allow"/{print NR; exit}' <<<"$ORDER_BLOCK")
LAST_DENY=$(awk '/"effect": "deny"/{n=NR} END{print n}' <<<"$ORDER_BLOCK")
{ [ -n "$FIRST_ALLOW" ] && [ -n "$LAST_DENY" ] && [ "$FIRST_ALLOW" -lt "$LAST_DENY" ]; } ||
  fail "docs/global-setup.md example: deny rules must come AFTER the broad allows (last-match-wins)"

# no stale per-agent model overrides in live config (prose mentions in
# AGENTS.md/spec = corrective history, allowed)
if grep -rqn "deepseek-v4-flash" opencode.json .opencode/agents/ .opencode/rules/ 2>/dev/null; then
  fail "stale per-agent model override reference (agents follow the session model)"
fi

# state file parses and carries the primary flag contract
python3 -c "import json;json.load(open('state/session-state.json'))" ||
  fail "state/session-state.json is not valid JSON"
echo "✓ toolchain docs gates passed (ADR-011, CHANGELOG shape, last-match-wins order, no stale overrides, state JSON)"

echo "=== Docs check complete ==="
exit 0
