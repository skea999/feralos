#!/bin/sh
# history-scrub.sh — anonymize + English-only git history for this repository.
#
# Removes from ALL branches/history:
#   - machine-local identifiers (e.g. ~/projects)   [rule: repo identity skea999 is public and stays]
#   - Italian text                                   [rule: English-only repository]
# Also deletes graft-like refs/replace/* left by previous operations.
#
# Usage:
#   ./scripts/history-scrub.sh extract   # list Italian candidate lines still in history
#   ./scripts/history-scrub.sh rewrite   # apply mappings via git-filter-repo (rewrites ALL branches)
#   ./scripts/history-scrub.sh push      # force push all branches (+tags) — AUTHORIZED operation
#   ./scripts/history-scrub.sh verify    # fresh clone + grep verification
#
# Docs: docs/guides/audit.md
# Mappings: scripts/history-replacements.txt (extend: original==>english per line)

set -eu
cd "$(dirname "$0")/.."

MAP="scripts/history-replacements.txt"
REMAINING="tests/artifacts/audits/history-italian-remaining.txt"
MARKERS='english-scrub-markers'

case "${1:-help}" in
  extract)
    mkdir -p "$(dirname "$REMAINING")"
    # candidates = added lines across all branches that match Italian markers
    git log -p --all | grep -a '^+' | grep -av '^+++' | \
      grep -aE "$MARKERS" | sed 's/^+//' | sort -u > "$REMAINING" || true
    echo "Italian candidate lines: $(wc -l < "$REMAINING") -> $REMAINING"
    echo "For each real Italian line add a mapping to $MAP:"
    echo "  original line==>english line"
    ;;

  rewrite)
    command -v git-filter-repo >/dev/null 2>&1 || {
      echo "git-filter-repo not found. Install: pip3 install git-filter-repo"; exit 1; }
    [ -f "$MAP" ] || { echo "missing $MAP"; exit 1; }
    R=$(git remote get-url origin)
    echo "remote: $R"

    git filter-repo --force --replace-text "$MAP"

    # drop graft-like replace refs (hundreds accumulated from earlier operations)
    git for-each-ref --format='%(refname)' refs/replace | while read -r ref; do
      git update-ref -d "$ref"
    done

    git remote add origin "$R"
    echo "Rewrite done. Check remaining Italian candidates:"
    echo "  ./scripts/history-scrub.sh extract"
    echo "Then push: ./scripts/history-scrub.sh push"
    ;;

  push)
    git push -f origin --all
    git push -f origin --tags 2>/dev/null || true
    echo "forced push of all branches done"
    ;;

  verify)
    R=$(git remote get-url origin)
    D=$(mktemp -d)
    git clone -q "$R" "$D/verify"
    cd "$D/verify"
    echo "--- Italian matches in full history (expect 0):"
    git log --all -p | grep -acE "$MARKERS" || echo 0
    echo "--- machine-user matches (expect 0):"
    git log --all -p | grep -ac '~/projects' || echo 0
    echo "--- branches:"; git branch -a
    cd - >/dev/null
    rm -rf "$D"
    ;;

  *)
    grep -E '^  \./' "$0" | sed 's/^ *//'
    ;;
esac
