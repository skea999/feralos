# Overview

## Project

FeralOS — Alpine-based distribution. Goals: memory safety + security. Nix for glibc packages. Dinit replaces OpenRC later. Custom repo. Installer Phase 1.

## Stack

- Go 1.22 (installer)
- Shell (legacy reference)
- YAML config (`variables.yaml` + `internal_rules.yaml`)

## Architecture Principle

Standard core, pluggable data — installer logic never changes, only data changes. Storage layouts are Go packages. Essential packages hardcoded. Optional + rules in YAML. Parser same code, only data changes. Booster replaces mkinitfs. UKI replaces GRUB. LUKS2 on RAID/Swap. Btrfs RAID1 on LUKS. No backward compatibility — installer runs once, always fresh.

Ref: `AGENTS.md#Architecture Principle`.

## Rules Tunables (stage-2 rules-ification)

The installer is tunable by editing ONLY `internal_rules.yaml`; the mechanism stays Go (2026-11-07). Tunable keys: `standardUserGroups`, `dinitSkipSessionManaged`, `feralosRepo.keyringUrl`, `kernel.{package,flavor}`, `microcode.{amd,intel}.{package,image}`, `boosterConfig.{compression,extraFiles,mountTimeout,network.dhcp}` (/etc/booster.yaml — sub-block mirror, byte-identical consumer), `edgeRepositories`, `resolvNameservers`, `maintenanceCron.{apkAudit,trim,scrub,snapshots}`, `limits.{nofileDefault,nofileUser}`, `nix.{channel,registryRef}`, per-DE `userServices` + `greeterGroups`. `LoadInternalRules` validates the always-consumed ones (empty = load error); edge/nix/microcode are feature-gated (their Setup functions enforce their own contracts). Guard: `TestRulesCarryRequiredTunables`.

Desktop misplacements fixed behavior-preserving (2026-11-07): login-shell trick lives in `desktops/niri.go`/`niri_wayle.go` step 4b (users.go is DE-agnostic — generic fish+chsh path runs for ALL DEs), turnstiled enablement is driven by the DE `services` map via services.go (its `/etc/dinit.d` fallback covers the turnstiled unit), `ensureIncusAdminGroup` + the `usermod -aG audio` duplicates removed (setupUserGroups creates missing rules groups; rules `groups` already carry audio), the `/var/log/ursine` belt-and-braces mkdir lives in misc.go (`setupUrsineLogDir`), and `AppendUnique` is exported from utils/logic (single shared copy). Generated output is BYTE-IDENTICAL for every DE (diff-proven per consumer).

## Error Handling (`unattended` vs `ignoreErrors`)

**`unattended: true`** skips manual input prompts ONLY (wizard skip; unchanged semantics). **`ignoreErrors: true`** (default `false`) = best-effort install: command failures are logged (`[ERROR][IGNORED] command failed (ignoreErrors=true): ...`) and installation CONTINUES. Wiring: `LoadConfig` sets `SKEA_IGNORE_ERRORS=1` when `conf.User.IgnoreErrors`; `shell.ExecuteCmd`/`ExecuteCmdArgs` honor it after retries are exhausted (return nil error → callers continue). Used by the test harness: harness runs are ALWAYS best-effort (`LoadVariablesFile` forces `cfg.IgnoreErrors = true`, `writeHarnessVariables` writes `ignoreErrors: true`, all fixtures carry `ignoreErrors: true`) — a failing optional step must not abort the run; the verification phase judges what actually works.

Details: `docs/guides/variables.md` (Options table), `src/utils/config/config.go` (env wiring), `src/utils/shell/execute.go` (ignore path).

## Project Structure

Brief tree — full tree in `docs/INSTALLER.md#Project Structure`:

```text
feralos_installer/
├── src/
│   ├── main.go, go.mod, variables.yaml
│   ├── variables/ (variables.yaml, internal_rules.yaml)
│   ├── utils/
│   │   ├── config/config.go
│   │   ├── shell/execute.go
│   │   ├── logic/logic.go
│   │   ├── swap/swap.go
│   │   └── createConfig/
│   ├── hostPhase/
│   │   ├── host_phase.go
│   │   ├── internal/ (bootstrap.go, bind_mounts.go, chroot_env.go, cleanup.go, hooks.go)
│   │   ├── features/ (install_config.go, secureboot.go, tpm.go)
│   │   └── disks/ (btrfsStandard/, btrfsRaid0/, btrfsStandardServer/, btrfsUnencrypted/)
│   ├── firstBoot/ (secureboot/main.go, tpm/main.go)
│   └── chrootPhase/
│       ├── chroot_phase.go
│       ├── internal/ (packages.go, boot.go, storage.go, uki.go, ...)
│       ├── features/ (hardening.go, zswap.go, nix.go, ...)
│       └── desktops/ (desktops.go)
├── tests/
│   ├── fixtures/variables/ (30 fixtures, full template mirror)
│   ├── harness/ (cmd/skea_harness, internal/)
│   └── artifacts/ (gitignored)
├── docs/ (PROJECT.md, INSTALLER.md, guides/, architecture/, adr/, harness/, intent/, ideas/)
├── state/session-state.json
└── handoff/latest.md
```

## Key Decisions

- `hostPhase` writes files (no chroot), `chrootPhase` runs commands inside chroot
- Module isolation, data-driven YAML/Go
- Security first: LUKS2, Secure Boot, AppArmor
