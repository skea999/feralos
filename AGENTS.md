# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — Alpine-based distribution, memory safety + security, Nix for glibc, Dinit selectable (OpenRC fallback), custom repo
**Status**: Phase 1 refining · Phase 2 shipping (niriWayle flagship desktop, Dinit selectable)

> **Primary context for AI agents.** This file is the lean index: context, rules of engagement, pointers. Full detail lives in `docs/` (canonical per topic). Read this file first, then follow the `→` pointers.
## Work history

All per-round work summaries (formerly "Recent Rounds") live in [CHANGELOG.md](CHANGELOG.md) — see the "Session work log" section at the end. Keep new round summaries there; this file stays lean context.

## Workflow

**Chain (complex)**: `context/recall` → `interview/refine` → `@planner` → *approve* → `@codebase` → PARALLEL(`@review`+`@code-reviewer`+`@vulnerability-scan`+`@information-scan`) → `@debugger/tests` → `@docs/docs` → save. @review accepts lens briefs (requirements|code|security|ux-responsive — `orchestrator-reference.instructions.md` Pattern 8); security lens loads `security-audit`.
**Direct (simple)**: a Tab-selectable agent direct — `codebase`, `planner`, `legal-advisor`, `researcher`, `blogger`, `em-advisor`, `orchestrator` (specialists are subagents: ask a primary agent to dispatch). High-stakes decision → `decision-grounding` skill. Architecture decision → `adr` skill.

## Changelog Policy

No change log here. Changes go to `CHANGELOG.md` (curated dated entries at the top + "Session work log" at the end), newest first. This file = stable context + policy only.

State: `state/session-state.json` (**primary** — full working state, read by the agents-opencode plugin context hook; write all future updates HERE) | Handoff: `handoff/latest.md`

State-file protection: the agents-opencode installer writes `state/session-state.json` only when missing (write-once-if-missing, code-proven + live-tested 2026-11-13) — it never overwrites. Git tracks the file: any accidental change is visible as a diff and restorable with `git checkout -- state/session-state.json`.

**Installer Workflow**: Understand Requirements (`CONTEXT.md`, `ROADMAP.md`, shell ref) → Plan (`hostPhase`/`chrootPhase` modules) → Implement (Go, naming conventions) → Test locally → Test harness (QEMU) → Review → Commit (conventional)

Key Decisions: `hostPhase` writes files (NO chroot), `chrootPhase` runs commands, module isolation, data-driven YAML/Go, security first (LUKS2, Secure Boot, AppArmor)

→ Details: `docs/guides/development.md` (how to code / separation), `docs/INSTALLER.md` (flow)

## Documentation Rule (MANDATORY)

**Every code change MUST update documentation.** The matching `docs/` file (architecture/adr/guides/harness/intent/ideas) is the CANONICAL detail home — write the full detail there; `AGENTS.md` stays a lean index and only needs its pointer/summary line adjusted. No code without doc update. Update the `Docs Index` when you add/move files. Harness docs → `docs/harness/` (canonical) — `tests/harness/README.md` stays pointer only. Work history → `CHANGELOG.md`.

**ADRs:** If you make an architectural decision (new pattern, isolation rule, storage layout, boot method, logging, compatibility, etc.) and no ADR exists, **write one** in `docs/adr/ADR-XXX-title.md` (next number) + update `docs/adr/README.md` index. Template: Status / Date / Context / Decision / Consequences. No decision without ADR.

→ Details: `docs/architecture/`, `docs/adr/`, `docs/guides/`, `docs/harness/`, `docs/intent/`, `docs/ideas/`

## Session Memory

Storage `.agent-memory/`, scripts `.agents/skills/session-memory/scripts/`

**The memory must be pushed.** `.agent-memory/` is tracked in git — after saving, commit and push it with the session's changes.

```bash
.agents/skills/session-memory/scripts/memory.sh context --days 14
.agents/skills/session-memory/scripts/memory.sh recall "vector" --limit 5
.agents/skills/session-memory/scripts/memory.sh stats
.agents/skills/session-memory/scripts/memory.sh save "decision" "Chose X because Y" tag1 tag2
MEMORY_IMPORTANCE=high .agents/skills/session-memory/scripts/memory.sh save "insight" "Key finding" installer
.agents/skills/session-memory/scripts/memory.sh save "summary" "Today: implemented X. Next: Z" -Tags installer go feralos
```

## Agents

### User-created (no manifest entry — safe from pack overwrites)
- `custom-researcher`

### Config entries in `opencode.json`
- `information-scan`, `vulnerability-scan` (empty objects — defaults)
- model overrides: none — every agent follows the model selected in opencode (removed 2026-11-13; the pinned `opencode-go/deepseek-v4-flash` became unavailable)

## Skills

- `.opencode/skills/` — agents-opencode pkg
- `.agents/skills/` — skills.sh CLI (43)

### User-modified (no auto-update)
- `session-memory`
- `using-agent-skills`

### User-created
None.

## Docs

- `docs/global-setup.md` — host + global environment setup (OpenCode V2)
- `docs/update.md` — update procedure for all components
- `docs/agents-guide.md` — agent roster + daily workflow

## Build & Test

```bash
# Build Go installer
cd src && go build -o ../feralos_installer .

# Build Go generator
cd src/utils/createConfig/createValues && go build -o ../../../../feralos_create_config .

# Build first-boot programs
cd src && go build -o ../feralos_secureboot ./firstBoot/secureboot/
cd src && go build -o ../feralos_tpm ./firstBoot/tpm/

# Build harness
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/

# Run installer (requires root + live)
sudo ./feralos_installer -config variables.yaml

# Run generator
./feralos_create_config

# Run harness tests
cd tests/harness
./skea_harness prepare-runner
./skea_harness run -f ~/Progetti/feralos/src/variables/variables.yaml
./skea_harness status
./skea_harness cleanup

# Run harness with fixture templates (covers all scenarios)
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_minimal.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_noswap.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_standard_swap_explicit_8g.yaml
ls tests/fixtures/variables/  # 30 fixtures, full variables.yaml template mirror (all scenarios)

# Run tests (when available)
cd src && go test ./...
```

→ Details: `docs/harness/commands.md` (CLI), `docs/harness/workflow.md` (3-phase flow), `docs/harness/artifacts.md` (test artifacts — gitignored, `.gitkeep` only, last-2-runs policy), `docs/guides/development.md` (naming conventions table + lint gotchas)

## Architecture Principle

Standard core, pluggable data — installer logic never changes, storage layouts are Go packages, essential packages hardcoded, optional+rules in YAML, parser same code only data changes, booster replaces mkinitfs, UKI replaces GRUB, LUKS2 on RAID/Swap, Btrfs RAID1 on LUKS. **No backward compatibility — installer runs once, always fresh.**

→ Details: `docs/architecture/overview.md` (principle + tree + decisions), `docs/adr/ADR-001-go-installer.md`, `ADR-005-luks-btrfs-booster-uki.md`, `ADR-006-no-backward-compat.md`

## Entry Point & Phase Isolation

`main.go` is entry point ONLY — it collects/generates config then delegates: `cleanup.Cleanup` (pre-host) → `hostPhase.Run` (disk layout switch by `conf.User.DiskLayout` → mount virtual FS → Alpine bootstrap → chroot prep) → `chrootPhase.Run` → `firstboot.WriteInstallConfig` → `firstboot.SetupSecureboot`/`SetupTpm` → `cleanup.Cleanup` (final unmount). `main` must NOT call disk layouts or chroot commands directly.

**Internal package rule**: `hostPhase/internal/*` is importable ONLY by `hostPhase` (and its subpackages). `main` cannot import it. This is the structural guarantee of host/chroot isolation.

→ Details: `docs/architecture/isolation.md` (entry-point + internal rule + hostPhase fixed order), `docs/INSTALLER.md#installation-flow`, `docs/adr/ADR-002-host-chroot-isolation.md`, `ADR-003-internal-package-rule.md`

## Logging Rule

**Commands: `shell.ExecuteCmd`; file writes: `shell.WriteFile`/`shell.WriteFileAppend` — in `hostPhase`/`chrootPhase`, never raw `os.*`; `import "os"` forbidden there (exceptions: `firstBoot/` standalone, `utils/shell` impl).** The `echo '<escaped>' > path` shell pattern is forbidden for new code. Documented secret exceptions (never logged): passwd via stdin pipe (chrootPhase/internal/users.go), cryptsetup stdin (hostPhase/disks/common/luks.go).

→ Details: `docs/architecture/logging.md`, `docs/adr/ADR-004-executecmd-logging.md`, `src/utils/shell/execute.go`, `src/utils/shell/write.go`

## Error Handling (`unattended` vs `ignoreErrors`)

**`unattended: true`** skips manual input prompts ONLY. **`ignoreErrors: true`** (default `false`) = best-effort install: failures logged (`[ERROR][IGNORED] ...`) and install CONTINUES (env `SKEA_IGNORE_ERRORS=1`, honored by `shell.ExecuteCmd`/`WriteFile`). Harness runs are ALWAYS best-effort — verification judges what works.

→ Details: `docs/architecture/overview.md#Error Handling`, `docs/guides/variables.md` (Options table), `src/utils/config/config.go`, `src/utils/shell/execute.go`

## Activatable Feature Confinement

**Rule: every activatable feature MUST be confined to a single owner file** under `src/chrootPhase/features/`, `src/utils/firstboot/`, or `src/chrootPhase/desktops/`; callers (`packages.go`, `logic.go`, `chroot_phase.go`, `host_phase.go`) must NOT contain feature logic. NEVER mutate feature flags from outside the owner file (exception: `NixPackages()` auto-enables internally).

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `utils/firstboot/secureboot.go` | `main.go` |
| `EnableTPM` | `utils/firstboot/tpm.go` | `main.go` |
| `EnablePodman` | `chrootPhase/features/podman.go` | `packages.go` + `chroot_phase.go` |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableEdgeRepos` | `chrootPhase/features/edgerepos.go` | `chroot_phase.go` (step 17, chroot tail — immediately after `SetupDbusAudit`) |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout layer) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` | `chroot_phase.go` |

→ Details: `docs/architecture/feature_confinement.md` (callers, new-feature pattern, user-session daemon rule, per-DE inventory incl. niri/niriWayle flagship facts, dinit service conventions)

## Rules Tunables (stage-2 rules-ification)

**The installer is tunable by editing ONLY `internal_rules.yaml`; the mechanism stays Go.** Tunable keys: `standardUserGroups`, `dinitSkipSessionManaged`, `feralosRepo.keyringUrl`, `kernel.{package,flavor}`, `microcode.{amd,intel}`, `boosterConfig.*`, `edgeRepositories`, `resolvNameservers`, `maintenanceCron.*`, `limits.*`, `nix.{channel,registryRef}`, per-DE `userServices` + `greeterGroups`. `LoadInternalRules` VALIDATES the always-consumed ones (empty = load error); edge/nix/microcode are feature-gated. Guard: `TestRulesCarryRequiredTunables`.

→ Details: `docs/architecture/overview.md#Rules Tunables (stage-2 rules-ification)` (full key list, misplacement moves, byte-identical proof)

## D-Bus Activation Rule (ADR-010)

**Rule: every D-Bus-activatable daemon is a dinit service; activation only triggers dinit** — no exceptions (user directive 2026-10-18). Unit naming follows WHO activates the service (R1, amended 2026-10-25): session/user = canonical bus name + `.user`; system services WE boot-enable (an OpenRC initd exists) = exactly the OpenRC initd name — the `services` map alone drives both inits; `dinitServices` stays EMPTY (mechanism kept for exceptional cases only); system dbus-activated on demand = canonical bus name. Supervision packages are NEVER listed in `internal_rules.yaml` — `feralos-dbus-audit` installs them (invoked TWICE at install: step 8a INSTALL pass + step 16 REPORT pass at the chroot tail; package name = the daemon's apk package via `apk info --who-owns`, never the bus name).

→ Full mechanism + checklist + diagnostics: `docs/architecture/dbus-activation.md` · decision: `docs/adr/ADR-010-dbus-activation-dinit.md`

## aports Source Pinning

**Rule (2026-11-08): every GitHub source in `feralos-aports` pins to a TAG archive, NEVER a commit archive** — sha512 = integrity guard, tag = version contract (re-published tag → checksum fails → diff and port the fix).

→ Full policy + workflow + inventory: `docs/aports-pinning.md` (canonical: `feralos-aports` `README.md` "Source pinning policy")

## Compatibility

No backward compat. Breaking changes allowed without fallback (e.g., breaking changes to `internal_rules.yaml` without deprecated path). Installer is single-run.

→ Details: `docs/adr/ADR-006-no-backward-compat.md`

## FirstBoot Architecture

Activatable features (Secure Boot, TPM) use standalone autonomous binaries in `src/firstBoot/`: installer compiles → copies to `/opt/feralos/bin/feralos-<name>` → writes `/opt/feralos/config.yaml` (no secrets); the user runs the binaries manually at first boot. 3-boot harness flow: runner (Debian, install) → target first boot (binaries configure features, bind LUKS to TPM2) → target verification. swtpm always passed (even with EnableTPM=false). Known limitation: Secure Boot enrollment not confirmable in QEMU; TPM auto-unlock works reliably.

→ Details: `docs/architecture/firstboot.md`, `docs/guides/first_boot.md` (real hardware), `docs/guides/podman.md`

## Project Structure

Full tree → `docs/INSTALLER.md#project-structure` and `docs/architecture/overview.md#project-structure`.

## Docs Index (docs/ = canonical detail source; AGENTS.md = index)

- **Architecture**: `docs/architecture/overview.md` (principle + tree + Rules Tunables + error handling), `isolation.md` (entry-point + internal rule + hostPhase fixed order), `logging.md` (ExecuteCmd rule), `feature_confinement.md` (single-owner rule + map + user-session daemon rule + per-DE inventory), `storage.md` (layouts: btrfsStandardServer, btrfsStandard, btrfsRaid0, btrfsUnencrypted), `firstboot.md` (secureboot/tpm 3-boot), `dbus-activation.md` (ADR-010 mechanism + new-service checklist + diagnostics), `boot-oneshots.md` (both-init boot oneshot pattern)
- **ADRs**: `docs/adr/README.md` → ADR-001 Go installer, 002 isolation, 003 internal rule, 004 ExecuteCmd, 005 LUKS+Btrfs+booster+UKI, 006 no compat, 007 dinit shutdown ordering, 008 dinit config files, 009 dinit service conventions, ADR-010 dbus activation→dinit, 011 docs canonical model
- **Guides** (user guides only): `docs/guides/README.md` (index) → `getting_started.md` (real hardware), `first_boot.md` (secureboot/tpm post-install), `variables.md` (variables.yaml compilation), `niriwayle.md` (flagship niriWayle desktop — Wayle/Veila/Raffi/oo7 + PAM unlock), `niri.md` (niri + NetworkManager + wayvnc), `podman.md` (btrfs driver), `development.md` (how to code / separation)
- **Harness**: `docs/harness/README.md` (index) → `overview.md` (what it does), `workflow.md` (3-phase + 3-boot), `artifacts.md` (where generated, gitignored), `building.md` (where compiled binary goes), `development.md` (how to code), `commands.md` (CLI+MCP), `debug_run.md` (`tests/debug_run/` boot existing run, LUKS inject). `tests/harness/README.md` is pointer only.
- **Intent / Ideas**: `docs/intent/vision.md`, `docs/intent/goals.md`, `docs/intent/vocabulary.md`, `docs/ideas/backlog.md` ← `CONTEXT.md` / `ROADMAP.md`
- **Project context / inventory**: `docs/memory_safe_alternatives.md` (detailed per-package memory-safety inventory; condensed table → root `CONTEXT.md`), `docs/FERALOS_NAME.md` (name rationale), `docs/aports-pinning.md` (feralos-side copy of the aports source-pinning policy — canonical + full: `feralos-aports` `README.md` "Source pinning policy")
- **AI tooling environment** (how this repo's agents/opencode setup is installed and updated — not FeralOS product docs): `docs/agents-guide.md` (agent roster + daily workflow), `docs/global-setup.md` (global opencode/caveman/cavecrew bootstrap — canonical copy from the base template), `docs/update.md` (update procedures)

→ AI agents: read AGENTS.md first for the map, then follow `→` pointers into `docs/` for the full detail.

## Roadmap

Full pipeline + priorities → `ROADMAP.md`. Memory-safe alternatives table → `CONTEXT.md`. Backlog → `docs/ideas/backlog.md`. Status snapshot: Phase 1 completed (layouts, generator, harness, niri, niriWayle flagship); Phase 1 refining in progress; Phase 2 dinit + custom repo live, full release planned.

## Known Issues

Living list → `CHANGELOG.md` "Known issues" section (top of the file). Work history → `CHANGELOG.md` "Session work log".
