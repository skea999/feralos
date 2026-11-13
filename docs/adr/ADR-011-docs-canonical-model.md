# ADR-011: Docs Canonical Model — docs/ holds the detail, AGENTS.md is the index

## Status

Accepted

## Date

2026-11-13

## Context

`AGENTS.md` had grown to 1076 lines duplicating every `docs/` section (the old "double documentation" rule: AGENTS.md keeps full info, docs/ copies). Every agent session loaded the full duplication on every prompt; the two copies drifted (the same rule text maintained twice), and the per-DE inventory alone was ~60 dense lines. The agents-opencode base template (skea-store) uses the opposite shape: a lean `AGENTS.md` (context + policy + pointers) with the full detail in topic docs, and `CHANGELOG.md` as the only work log.

## Decision

- **`docs/` = canonical detail source.** Every architecture rule, tunable, convention and inventory lives in its topic doc (`docs/architecture/*`, `docs/guides/*`, `docs/harness/*`, `docs/adr/*`).
- **`AGENTS.md` = lean index** (≤ ~220 lines): project context, workflow rules of engagement, operational commands, pointers. A section may keep a short rule statement + table when it is needed on every session (e.g. the feature confinement map, the D-Bus rule summary with the docs_check markers); the deep detail always lives in the doc.
- **`CHANGELOG.md` = the only work log**: curated dated entries at the top (Keep-a-Changelog, newest first) + "Session work log" section at the end for the detailed per-round summaries. No work history in AGENTS.md.
- **Documentation Rule rewritten**: a code change updates the matching `docs/` file (canonical) and, only if needed, the AGENTS.md pointer/summary line — not both full copies.
- **Enforcement**: `scripts/docs_check.sh` indexes every `docs/` file (orphan check) and spot-checks the AGENTS.md ↔ docs drift markers.

## Consequences

- Agents read a ~215-line index instead of a 1076-line duplicate; token cost per session drops and drift becomes structurally impossible (single copy per fact).
- New detail must be written in `docs/`, not in AGENTS.md — the index only carries pointers.
- The order of `permissions` rules follows last-match-wins (broad allows first, specific denies last); the same semantics apply to the global config and are documented in `docs/global-setup.md`.

## References

- `AGENTS.md#Documentation Rule (MANDATORY)` (rewritten)
- `CHANGELOG.md` (Known issues + Session work log)
- Base template: `skea-store` repo, branch `opencode-template`
