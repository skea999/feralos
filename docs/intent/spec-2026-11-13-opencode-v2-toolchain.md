# Spec — OpenCode V2 Toolchain Migration & Base Alignment (2026-11-13)

Retro-spec + plan + workflow execution log for the tooling round completed on 2026-11-13
(per the Documentation Rule this is the canonical record; AGENTS.md carries only the pointer).

## 1. Requirements ledger (interview step — directives as given, delivered status)

| Directive (user, verbatim intent) | Delivered |
|---|---|
| Migrate to opencode V2 keeping old sessions | ✓ binary 2.2.6, configs/plugins/agents migrated, last V1 session + 58 subagents imported |
| Update project + global things, ensure they work on V2 | ✓ pack 4.3.5, caveman V2-native, opencode-rules verified, live reload tests |
| Only the last V1 session matters; if problems, import only that one | ✓ parent + 58 children imported, round-trip verified |
| Investigate why nix shows V1 while opencode is V2 | ✓ registry = nixpkgs-unstable; stable channel still ships 1.x |
| Update all skills/agents/local things via npm first, then fix for V2 | ✓ npx update flow + post-update V2 fixes (documented in docs/update.md) |
| opencode-rules plugin is critical — check it well | ✓ verified V2-native (dist code: ctx hooks + event subscribe + dispose) |
| Local harness (Go) untouched | ✓ not modified |
| Push local harness to git | ✓ (superseded: intended the pack files; all committed) |
| workflow rule: only the last session, import it | ✓ done |
| Move AGENTS.md changelogs to changelog.md — the correct place | ✓ CHANGELOG.md "Session work log" |
| Check rules, skills in agents, docs; compare AGENTS.md with the new base | ✓ done (rules identical except richer workflow.md — adopted; skills same pack version) |
| AGENTS.md still too big — ADR-010 detail belongs in its folder, only the reference here | ✓ ADR-010 = summary + pointer; full mechanism in docs/architecture/dbus-activation.md |
| session-state.json must not be overwritten by updates; all info must be preserved | ✓ write-once-if-missing proven (code + live test); full 99KB content verbatim |
| Check if session_state.json is referenced anywhere; fix; keep one file only | ✓ zero refs, old file deleted (git-recoverable) |
| Update docs with same names from skea-store; update common AGENTS.md sections | ✓ renamed hyphenated + replaced; Workflow/Changelog-Policy/Agents/Skills/Docs adopted |
| Adopt ALL workflow steps, even the skipped ones; audit "done" claims | ✓ this document + the review/security passes |
| Fix custom-researcher / researcher from skea-store | ✓ both adopted |
| Apply the same corrections to skea-store, not the rules | ✓ docs fixes pushed (f40297e, 8af55ec, 96417a2) |

## 2. Spec (target state)

- OpenCode CLI 2.x (nix unstable) as the only install; background service owns sessions/plugins/permissions.
- Config files V2-only: `permissions` ordered arrays (last-match-wins — broad allows first, specific denies last), `agents`/`plugins`/`providers`/`mcp.servers` maps, `cli.json` for the TUI.
- `.opencode/` pack at 4.3.5 with V2 frontmatter everywhere; global env = caveman (V2 `server.js` + `index.js` shim) + `opencode-rules` + `context-mode` + ast-grep skills.
- Docs model: `docs/` canonical, `AGENTS.md` lean index (≤~220 lines), `CHANGELOG.md` = only work log (+ Known issues).
- State: `state/session-state.json` primary, installer write-once-protected, git-restorable.

## 3. Plan (as executed)

1. Migrate binary + configs → 2. verify models/plugins/agents live → 3. migrate sessions (export→convert→import) → 4. update packs via npm/npx (node24) → 5. re-apply V2 fixes → 6. docs restructure (CHANGELOG move + lean AGENTS.md + docs enrichment) → 7. base alignment from skea-store → 8. state consolidation → 9. workflow compile + parallel review → 10. fix findings → 11. commit/push per repo.

## 4. Decision grounding (sources)

- Official V2 docs: migrate-v1 (plugin API breaking changes), permissions (last-match-wins, ordered array), agents (modes primary/subagent/all, frontmatter fields), config, build/plugins/migrate-v1 (hook mapping table), build/plugins/server (ctx domains) — fetched from opencode.ai/v2/docs during the session.
- V2 OpenAPI (live server `/openapi.json`): session import/export contract, `Session.Info` required keys (`id, projectID, cost, tokens, time, location`).
- V1 ground truth: store binary `opencode-1.18.34` export output; nixpkgs PR/issue trail for the `opencode-stable.db` channel naming.

## 5. Blast-radius check (code-change impact)

Changed: docs/**, .opencode/** (pack + frontmatter), opencode.json, AGENTS.md, CHANGELOG.md, state/, handoff/, global `~/.config/opencode/*` (outside repo).
Affected consumers verified: `scripts/docs_check.sh` (extended), opencode plugin loader (3 plugins loaded, log-verified), agent discovery (`debug agents`), rules injection (live in session), state file plugin context hook (reload-tested), pre-commit information-scan (every commit), CI (untouched — docs only).

## 6. Debug log (issues found → fixed)

1. V1 binary on the channel DB marker → "Database is not empty and has no session table" → isolated `XDG_DATA_HOME` copy.
2. V2 import schema rejections → converter mapping (location, inline content[], tool `time.created`).
3. Import landed in `/home/skem` → request-level `location` required.
4. Global-config relative plugin path silently skipped → absolute path + `index.js` shim.
5. Subagents failed `Model unavailable: opencode-go/deepseek-v4-flash` → per-agent model overrides removed (agents follow the session model).
6. **F1 (review)**: deny-first permission order = dead denies under last-match-wins → reordered in docs (both repos) + live global config.

## 7. Workflow execution log (tracking)

| Step | Status | Artifact / evidence |
|---|---|---|
| 0 context+recall | DONE | memory.sh context/recall run; `.agent-memory/2026/11/13.jsonl` |
| 1 interview / idea-refine | DONE | §1 requirements ledger (directives → delivered) |
| 2 spec | DONE | §2 this document (docs/intent, canonical) |
| 3 plan (+decision-grounding) | DONE | §3 plan + §4 sources |
| 4 impact check + implement | DONE | §5 blast radius; all changes implemented |
| 4.1 parallel review on diff | DONE | tooling round: CHANGES REQUESTED → fixed; FeralOS product round (`06de0be^..68b2873`, frozen `.gate/diff.patch`): APPROVE WITH NITS → nits applied, F2 tracked in state |
| 5 test-driven | DONE | `scripts/docs_check.sh` extended with the new gates (orphan/adr/single-H1/permission-order/stale-refs) — run green |
| 6 debugging | DONE | §6 log (6 issues diagnosed + fixed) |
| 7 quality + security + performance | DONE | @code-reviewer (quality) + @review security lens on the permission rules (this round); performance N/A (docs) |
| 8 code-simplification | DONE | AGENTS.md 1076→~220 lines; single state file; dedup removed |
| 9 grill-with-docs (+adr) | DONE | docs updated in-step; **ADR-011** written + indexed |
| 10 git workflow | DONE | conventional commits, both repos pushed |
| 11 CI/CD automation | DONE | docs_check extensions run in pre-commit + CI path |
| 12 deprecation/migration | DONE | `docs/update.md` migration recipe; V1 keys deprecated in config |
| 13 save summary | DONE | memory.sh summary saved (final update after this round) |
